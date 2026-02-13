package analyzer

import (
	"fmt"
	"strings"
	"sync"
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

// condPriority defines the order in which job conditions are evaluated.
// The first condition with Status=True in this list wins.
var condPriority = []kubeflowv1.JobConditionType{
	kubeflowv1.JobFailed,
	kubeflowv1.JobRestarting,
	kubeflowv1.JobSucceeded,
	kubeflowv1.JobSuspended,
	kubeflowv1.JobRunning,
	kubeflowv1.JobCreated,
}

// findActiveCondition returns the highest-priority condition whose Status is True,
// or nil if none is active.
func findActiveCondition(conds []kubeflowv1.JobCondition) *kubeflowv1.JobCondition {
	for _, pt := range condPriority {
		for i := range conds {
			if conds[i].Type == pt && conds[i].Status == v1.ConditionTrue {
				return &conds[i]
			}
		}
	}
	return nil
}

type PytorchJobAnalyzer struct {
	prometheus *prom.PromAPI
	client     kfclientset.Interface
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

	// === Job Condition 分析 ===
	skipPodAnalysis := false
	active := findActiveCondition(job.Status.Conditions)
	if active == nil {
		result.Metadata["JobStatus"] = "Unknown"
		result.Warning = append(result.Warning, common.Warning{
			Text: "Job has no active condition",
		})
	} else {
		result.Metadata["JobStatus"] = string(active.Type)
		switch active.Type {
		case kubeflowv1.JobSucceeded:
			result.Info = append(result.Info, common.Info{
				Text: "Job completed successfully.",
			})
			skipPodAnalysis = true
		case kubeflowv1.JobFailed:
			result.Error = append(result.Error, kcommon.Failure{
				Text: fmt.Sprintf("Job failed: %s - %s", active.Reason, active.Message),
			})
		case kubeflowv1.JobRestarting:
			result.Warning = append(result.Warning, common.Warning{
				Text: fmt.Sprintf("Job is restarting: %s - %s", active.Reason, active.Message),
			})
		case kubeflowv1.JobSuspended:
			result.Info = append(result.Info, common.Info{
				Text: fmt.Sprintf("Job is suspended: %s", active.Reason),
			})
			skipPodAnalysis = true
		case kubeflowv1.JobRunning, kubeflowv1.JobCreated:
			result.Info = append(result.Info, common.Info{
				Text: fmt.Sprintf("Job is %s.", active.Type),
			})
		}
	}

	// === Job Events 分析 ===
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

	// === Replica spec 采集 ===
	result.Metadata["MasterExpected"] = "0"
	result.Metadata["WorkerExpected"] = "0"
	specs := job.Spec.PyTorchReplicaSpecs
	if masterSpec, ok := specs["Master"]; ok && masterSpec.Replicas != nil {
		result.Metadata["MasterExpected"] = fmt.Sprintf("%d", *masterSpec.Replicas)
	}
	if workerSpec, ok := specs["Worker"]; ok && workerSpec.Replicas != nil {
		result.Metadata["WorkerExpected"] = fmt.Sprintf("%d", *workerSpec.Replicas)
	}

	// === 只有非 Succeeded/Suspended 才下沉 Pod 分析 ===
	if !skipPodAnalysis {
		if err := p.analyzePytorchJobPods(a, job, result); err != nil {
			klog.Warningf("analyze pods for %s/%s failed: %v", a.Namespace, job.Name, err)
		}
	}

	return result, nil
}

// categorizeWorkers splits worker pods into abnormal and normal buckets.
// Abnormal: Failed, Pending, Unknown, or Running but not Ready.
func categorizeWorkers(workerPods []*v1.Pod) (abnormal, normal []*v1.Pod) {
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
					break
				}
			}
			if !ready {
				isAbnormal = true
			}
		}
		if isAbnormal {
			abnormal = append(abnormal, wp)
		} else {
			normal = append(normal, wp)
		}
	}
	return
}

// analyzePodWithExplain diagnoses a single Pod.
//
//   - Explain mode (a.AIClient != nil): calls Pod-level LLM and returns a
//     structured conclusion (~4 lines).
//   - Non-explain mode: falls back to Summarize() raw text.
//
// All relevant fields from the parent Analyzer are forwarded to the sub-analyzer.
func (p PytorchJobAnalyzer) analyzePodWithExplain(
	a common.Analyzer,
	pod *v1.Pod,
	podAnalyzer PodAnalyzer,
) string {
	sub := common.Analyzer{
		Analyzer: kcommon.Analyzer{
			Client:    a.Client,
			Context:   a.Context,
			Namespace: a.Namespace,
			AIClient:  a.AIClient,
		},
		Name:           pod.Name,
		CollectorImage: a.CollectorImage,
		EnableProm:     a.EnableProm,
		EnablePodLog:   a.EnablePodLog,
		PodLogConfig:   a.PodLogConfig,
	}

	r, err := podAnalyzer.Analyze(sub)
	if err != nil || r == nil {
		return fmt.Sprintf("Pod %s: analysis failed: %v", pod.Name, err)
	}

	if a.AIClient != nil {
		prompt := podAnalyzer.Prompt(r)
		if prompt != "" {
			if explain, err := a.AIClient.GetCompletion(a.Context, prompt); err == nil {
				return explain
			} else {
				klog.Warningf("pod LLM for %s failed: %v", pod.Name, err)
			}
		}
	}

	return podAnalyzer.Summarize(r)
}

func (p PytorchJobAnalyzer) analyzePytorchJobPods(a common.Analyzer, job *kubeflowv1.PyTorchJob, result *common.Result) error {
	const maxDetailedAbnormalWorkers = 5

	labelPrefix := "training.kubeflow.org/"
	labelSelector := fmt.Sprintf("%sjob-name=%s", labelPrefix, job.Name)

	pods, err := a.Client.GetClient().CoreV1().Pods(a.Namespace).List(a.Context, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("list pods failed: %w", err)
	}

	var masterPod *v1.Pod
	var workerPods []*v1.Pod
	masterCreatedCount := 0
	workerCreatedCount := 0

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

	result.Metadata["MasterCreatedCount"] = fmt.Sprintf("%d", masterCreatedCount)
	result.Metadata["WorkerCreatedCount"] = fmt.Sprintf("%d", workerCreatedCount)

	podAnalyzer := NewPodAnalyzer(p.prometheus)
	abnormal, normal := categorizeWorkers(workerPods)

	// Build the list of pods that need full analysis (master + up to N abnormal workers).
	type podTask struct {
		pod      *v1.Pod
		isMaster bool
	}
	var tasks []podTask
	if masterPod != nil {
		tasks = append(tasks, podTask{masterPod, true})
	}
	limit := min(len(abnormal), maxDetailedAbnormalWorkers)
	for _, wp := range abnormal[:limit] {
		tasks = append(tasks, podTask{wp, false})
	}

	// Concurrent analysis.
	explains := make([]string, len(tasks))
	var wg sync.WaitGroup
	for i, t := range tasks {
		wg.Add(1)
		go func(i int, t podTask) {
			defer wg.Done()
			explains[i] = p.analyzePodWithExplain(a, t.pod, podAnalyzer)
		}(i, t)
	}
	wg.Wait()

	// Distribute results.
	idx := 0
	if masterPod != nil {
		result.Metadata["MasterDiagnosis"] = explains[0]
		idx = 1
	}

	var workerLines []string
	for _, wp := range abnormal[:limit] {
		workerLines = append(workerLines,
			fmt.Sprintf("Worker Pod %s (Abnormal):\n%s", wp.Name, explains[idx]))
		idx++
	}

	// Normal workers: single summary line.
	if len(normal) > 0 {
		workerLines = append(workerLines,
			fmt.Sprintf("Other %d worker pod(s) are Running and Ready.", len(normal)))
	}

	if len(workerLines) > 0 {
		result.Metadata["WorkerDiagnosis"] = strings.Join(workerLines, "\n---\n")
	}

	return nil
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

	errorInfo := ""
	for _, e := range result.Error {
		errorInfo += e.Text + "\n"
	}
	eventInfo := ""
	for _, w := range result.Warning {
		eventInfo += w.Text + "\n"
	}
	logInfo := ""
	for _, i := range result.Info {
		logInfo += i.Text + "\n"
	}

	data := ai.PromptData{
		ErrorInfo: strings.TrimSpace(errorInfo),
		EventInfo: strings.TrimSpace(eventInfo),
		LogInfo:   strings.TrimSpace(logInfo),
		Metadata:  metadata,
	}

	prompt, err := ai.GetRenderedPrompt("PytorchJob", data)
	if err != nil {
		return fmt.Sprintf("Prompt rendering error: %v", err)
	}
	return prompt
}
