package analyzer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/k8sgpt-ai/k8sgpt/pkg/analyzer"
	kcommon "github.com/k8sgpt-ai/k8sgpt/pkg/common"
	"github.com/k8sgpt-ai/k8sgpt/pkg/util"
	kubeflowv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	kfclientset "github.com/kubeflow/training-operator/pkg/client/clientset/versioned"
	ai "github.com/scitix/aegis/pkg/ai"
	"github.com/scitix/aegis/pkg/analyzer/common"
	"github.com/scitix/aegis/pkg/prom"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type PytorchJobAnalyzer struct {
	prometheus   *prom.PromAPI
	client       kfclientset.Interface
}

func NewPytorchJobAnalyzer(prometheus *prom.PromAPI, client kfclientset.Interface) PytorchJobAnalyzer {
	return PytorchJobAnalyzer{
		client:     client,
		prometheus: prometheus,
	}
}

func (p PytorchJobAnalyzer) Analyze(a common.Analyzer) (*common.Result, error) {
	kind := "PytorchJob"

	analyzer.AnalyzerErrorsMetric.DeletePartialMatch(map[string]string{
		"analyzer_name": kind,
	})

	jobName := a.Name
	job, err := p.client.KubeflowV1().PyTorchJobs(a.Namespace).Get(a.Context, jobName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting PyTorchJob %s/%s: %w", a.Namespace, jobName, err)
	}

	result := &common.Result{
		Result: kcommon.Result{
			Kind: kind,
			Name: job.Name,
		},
		Metadata: map[string]string{
			"JobName": job.Name,
		},
	}

	// === Job Condition(failures result.Error) 分析 ===
	conds := job.Status.Conditions
	skipPodAnalysis := false

	if len(conds) > 0 {
		last := conds[len(conds)-1]
		result.Metadata["JobStatus"] = string(last.Type)
		switch {
		case last.Type == "Succeeded" && last.Status == "True":
			result.Info = append(result.Info, common.Info{
				Text: "Job completed successfully. No diagnosis needed.",
			})
			skipPodAnalysis = true
		case last.Type == "Failed" && last.Status == "True":
			msg := fmt.Sprintf("Job failed: %s - %s", last.Reason, last.Message)
			result.Error = append(result.Error, kcommon.Failure{Text: msg})
		case (last.Type == "Running" || last.Type == "Created") && last.Status == "True":
			// Running / Created → Info
			msg := fmt.Sprintf("Job is %s. Pods are still running or initializing.", last.Type)
			result.Info = append(result.Info, common.Info{Text: msg})
		default:
			// 其他状态 → Warning
			msg := fmt.Sprintf("Job is in unexpected state: %s - %s/%s", last.Type, last.Reason, last.Message)
			result.Warning = append(result.Warning, common.Warning{Text: msg})
		}
	}

	// === Job Events(warnings result.Warnings) 分析 ===
	rawEvents, err := FetchEvents(a.Context, a.EnableProm, p.prometheus, a.Client, "PyTorchJob", a.Namespace, job.Name, "Warning", "")
	if err != nil {
		klog.Warningf("fetch pytorchjob events failed: %v", err)
	} else {
		if a.EnableProm {
			for _, event := range rawEvents.([]prom.Event) {
				result.Warning = append(result.Warning, jobEventWarning(job.Name, event))
			}
		} else {
			for _, event := range rawEvents.([]v1.Event) {
				result.Warning = append(result.Warning, jobEventWarningLegacy(job.Name, event))
			}
		}
	}

	// === Replica、spec 状态采集 ===
	result.Metadata["MasterExpected"] = "0"
	result.Metadata["WorkerExpected"] = "0"
	specs := job.Spec.PyTorchReplicaSpecs
	if masterSpec, ok := specs["Master"]; ok && masterSpec.Replicas != nil {
		result.Metadata["MasterExpected"] = fmt.Sprintf("%d", *masterSpec.Replicas)
	}
	if workerSpec, ok := specs["Worker"]; ok && workerSpec.Replicas != nil {
		result.Metadata["WorkerExpected"] = fmt.Sprintf("%d", *workerSpec.Replicas)
	}

	// === 只有非 Succeeded 才下沉 Pod 分析 ===
	if !skipPodAnalysis {
		err := p.analyzePytorchJobPods(a, job, result)
		if err != nil {
			klog.Warningf("analyze pods for %s/%s failed: %v", a.Namespace, job.Name, err)
		}
	}

	return result, nil
}

func (p PytorchJobAnalyzer) analyzePytorchJobPods(a common.Analyzer, job *kubeflowv1.PyTorchJob, result *common.Result) error {
	labelPrefix := "training.kubeflow.org/"
	labelSelector := fmt.Sprintf("%sjob-name=%s", labelPrefix, job.Name)

	pods, err := a.Client.GetClient().CoreV1().Pods(a.Namespace).List(a.Context, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("list pods failed: %w", err)
	}

	masterCreatedCount := 0
	workerCreatedCount := 0
	var masterPod *v1.Pod
	var workerPods []*v1.Pod

	for i := range pods.Items {
		pod := &pods.Items[i]
		switch pod.Labels[labelPrefix+"replica-type"] {
		case "master":
			masterPod = pod
			masterCreatedCount++
		case "worker":
			workerPods = append(workerPods, pod)
			workerCreatedCount++
		}
	}

	result.Metadata["MasterCreatedCount"] = strconv.Itoa(masterCreatedCount)
	result.Metadata["WorkerCreatedCount"] = strconv.Itoa(workerCreatedCount)

	podAnalyzer := NewPodAnalyzer(p.prometheus)

	// Master 分析
	if masterPod != nil {
		r, err := podAnalyzer.Analyze(common.Analyzer{
			Analyzer: kcommon.Analyzer{
				Client:    a.Client,
				Context:   a.Context,
				Namespace: a.Namespace,
			},
			Name:           masterPod.Name,
			CollectorImage: a.CollectorImage,
		})
		if err == nil && r != nil {
			result.Metadata["MasterDiagnosis"] = podAnalyzer.Summarize(r)
		}
	}

	// Worker 分析
	workerDiagnosis := p.analyzeWorkerPods(a, workerPods)
	if workerDiagnosis != "" {
		result.Metadata["WorkerDiagnosis"] = workerDiagnosis
	}

	return nil
}

func (p PytorchJobAnalyzer) analyzeWorkerPods(a common.Analyzer, workerPods []*v1.Pod) string {
	const maxDetailedAbnormalWorkers = 5
	const maxRunningSummaryWorkers = 3

	podAnalyzer := NewPodAnalyzer(p.prometheus)

	var abnormalWorkers []*v1.Pod
	var normalWorkers []*v1.Pod

	// categorize
	for _, wp := range workerPods {
		isAbnormal := false
		switch wp.Status.Phase {
		case v1.PodFailed, v1.PodPending, v1.PodUnknown:
			isAbnormal = true
		case v1.PodRunning:
			ready := false
			for _, cond := range wp.Status.Conditions {
				if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
					ready = true
				}
			}
			if !ready {
				isAbnormal = true
			}
		}

		if isAbnormal {
			abnormalWorkers = append(abnormalWorkers, wp)
		} else {
			normalWorkers = append(normalWorkers, wp)
		}
	}

	var workerSummaries []string

	// === abnormal workers ===
	for i, wp := range abnormalWorkers {
		if i >= maxDetailedAbnormalWorkers {
			break
		}
		r, err := podAnalyzer.Analyze(common.Analyzer{
			Analyzer: kcommon.Analyzer{
				Client:    a.Client,
				Context:   a.Context,
				Namespace: a.Namespace,
			},
			Name:           wp.Name,
			CollectorImage: a.CollectorImage,
		})
		if err == nil && r != nil {
			workerSummaries = append(workerSummaries,
				fmt.Sprintf("Worker Pod %s (Abnormal):\n%s", wp.Name, podAnalyzer.Summarize(r)))
		}
	}

	// === normal workers ===
	if len(abnormalWorkers) == 0 {
		// No abnormal
		for _, wp := range normalWorkers {
			workerSummaries = append(workerSummaries,
				fmt.Sprintf("Worker Pod %s is Running and Ready.", wp.Name))
		}
	} else {
		// abnormal → normal limit
		for i, wp := range normalWorkers {
			if i >= maxRunningSummaryWorkers {
				break
			}
			workerSummaries = append(workerSummaries,
				fmt.Sprintf("Worker Pod %s is Running and Ready.", wp.Name))
		}
	}

	if len(workerSummaries) > 0 {
		return strings.Join(workerSummaries, "\n---\n")
	}
	return ""
}

func jobEventWarning(jobName string, event prom.Event) common.Warning {
	return common.Warning{
		Text: fmt.Sprintf("Job %s has %s event at %s %s(%s) count %d", jobName, event.Type, event.TimeStamps, event.Reason, event.Message, event.Count),
		Sensitive: []kcommon.Sensitive{
			{
				Unmasked: jobName,
				Masked:   util.MaskString(jobName),
			},
		},
	}
}

func jobEventWarningLegacy(jobName string, event v1.Event) common.Warning {
	timestamp := event.LastTimestamp.Time
	if timestamp.IsZero() {
		timestamp = event.EventTime.Time
	}
	return common.Warning{
		Text: fmt.Sprintf("Job %s has %s event at %s %s(%s) count %d", jobName, event.Type, timestamp.Format(time.RFC3339), event.Reason, event.Message, event.Count),
		Sensitive: []kcommon.Sensitive{
			{
				Unmasked: jobName,
				Masked:   util.MaskString(jobName),
			},
		},
	}
}

func (p PytorchJobAnalyzer) Prompt(result *common.Result) string {
	if result == nil {
		return ""
	}

	metadata := map[string]string{
		"JobName":            result.Metadata["JobName"],
		"JobStatus":          result.Metadata["JobStatus"],
		"MasterExpected":     result.Metadata["MasterExpected"],
		"WorkerExpected":     result.Metadata["WorkerExpected"],
		"MasterCreatedCount": result.Metadata["MasterCreatedCount"],
		"WorkerCreatedCount": result.Metadata["WorkerCreatedCount"],
		"MasterDiagnosis":    result.Metadata["MasterDiagnosis"],
		"WorkerDiagnosis":    result.Metadata["WorkerDiagnosis"],
	}

	data := ai.PromptData{
		ErrorInfo: strings.TrimSpace(func() string {
			s := ""
			for _, e := range result.Error {
				s += e.Text + "\n"
			}
			return s
		}()),
		EventInfo: strings.TrimSpace(func() string {
			s := ""
			for _, w := range result.Warning {
				s += w.Text + "\n"
			}
			return s
		}()),
		LogInfo: strings.TrimSpace(func() string {
			s := ""
			for _, i := range result.Info {
				s += i.Text + "\n"
			}
			return s
		}()),
		Metadata: metadata,
	}

	prompt, err := ai.GetRenderedPrompt("PytorchJob", data)
	if err != nil {
		return fmt.Sprintf("Prompt rendering error: %v", err)
	}
	return prompt
}
