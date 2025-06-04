package analyzer

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/k8sgpt-ai/k8sgpt/pkg/analyzer"
	kcommon "github.com/k8sgpt-ai/k8sgpt/pkg/common"
	"github.com/k8sgpt-ai/k8sgpt/pkg/kubernetes"
	"github.com/k8sgpt-ai/k8sgpt/pkg/util"
	"gitlab.scitix-inner.ai/k8s/aegis/pkg/analyzer/common"
	"gitlab.scitix-inner.ai/k8s/aegis/pkg/prom"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type PodAnalyzer struct {
	prometheus *prom.PromAPI
}

func NewPodAnalyzer() PodAnalyzer {
	return PodAnalyzer{
		prometheus: prom.GetPromAPI(),
	}
}

func (p PodAnalyzer) Analyze(a common.Analyzer) (*common.Result, error) {

	kind := "Pod"

	analyzer.AnalyzerErrorsMetric.DeletePartialMatch(map[string]string{
		"analyzer_name": kind,
	})

	name := a.Name

	// get pod
	pod, err := a.Client.GetClient().CoreV1().Pods(a.Namespace).Get(a.Context, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var failures []kcommon.Failure
	// Check for pending pods
	if pod.Status.Phase == "Pending" {
		// Check through container status to check for crashes
		for _, containerStatus := range pod.Status.Conditions {
			if containerStatus.Type == v1.PodScheduled && containerStatus.Reason == "Unschedulable" {
				if containerStatus.Message != "" {
					failures = append(failures, kcommon.Failure{
						Text:      containerStatus.Message,
						Sensitive: []kcommon.Sensitive{},
					})
				}
			}
		}
	}

	// Check for errors in the init containers.
	failures = append(failures, analyzeContainerStatusFailures(a, pod.Status.InitContainerStatuses, pod.Name, pod.Namespace, string(pod.Status.Phase))...)

	// Check for errors in containers.
	failures = append(failures, analyzeContainerStatusFailures(a, pod.Status.ContainerStatuses, pod.Name, pod.Namespace, string(pod.Status.Phase))...)

	if len(failures) > 0 {
		analyzer.AnalyzerErrorsMetric.WithLabelValues(kind, pod.Name, pod.Namespace).Set(float64(len(failures)))
	}

	var warnings []common.Warning
	// 全量 event
	events, err := p.prometheus.GetEventWithRange(a.Context, "Pod", pod.Namespace, pod.Name, "", "7d")
	if err != nil {
		klog.Warningf("error get pod events from prometheus: %s", err)
	} else {
		for _, event := range events {
			warnings = append(warnings, podEventWarning(pod.Name, event))
		}
	}

	var infos []common.Info
	findFunc := func(name string, containerStatuses []v1.ContainerStatus) *v1.ContainerStatus {
		for _, status := range containerStatuses {
			if name == status.Name {
				return &status
			}
		}

		return nil
	}

	for _, c := range pod.Spec.InitContainers {
		name := c.Name
		containerStatus := findFunc(name, pod.Status.InitContainerStatuses)
		if containerStatus == nil { // not created
			continue
		}

		logs, err := getContainerLogs(a.Context, a.Client, pod, &c, containerStatus)
		if err != nil || len(logs) == 0 {
			continue
		}
		infos = append(infos, common.Info{
			Text: fmt.Sprintf("pod %s init container %s logs: %s", pod.Name, name, strings.Join(logs, "\n")),
		})
	}

	for _, c := range pod.Spec.Containers {
		name := c.Name
		containerStatus := findFunc(name, pod.Status.ContainerStatuses)
		if containerStatus == nil { // not created
			continue
		}

		logs, err := getContainerLogs(a.Context, a.Client, pod, &c, containerStatus)
		if err != nil || len(logs) == 0 {
			continue
		}
		infos = append(infos, common.Info{
			Text: fmt.Sprintf("pod %s container %s logs: %s", pod.Name, name, strings.Join(logs, "\n")),
		})
	}

	result := &common.Result{
		Result: kcommon.Result{
			Kind:  kind,
			Name:  pod.Name,
			Error: failures,
		},
		Warning: warnings,
		Info:    infos,
	}

	parent, found := util.GetParent(a.Client, pod.ObjectMeta)
	if found {
		result.ParentObject = parent
	}

	return result, nil
}

func analyzeContainerStatusFailures(a common.Analyzer, statuses []v1.ContainerStatus, name string, namespace string, statusPhase string) []kcommon.Failure {
	var failures []kcommon.Failure

	// Check through container status to check for crashes or unready
	for _, containerStatus := range statuses {
		if containerStatus.State.Waiting != nil {
			if containerStatus.State.Waiting.Reason == "ContainerCreating" && statusPhase == "Pending" {
				// This represents a container that is still being created or blocked due to conditions such as OOMKilled
				// parse the event log and append details
				evt, err := util.FetchLatestEvent(a.Context, a.Client, namespace, name)
				if err != nil || evt == nil {
					continue
				}
				if isEvtErrorReason(evt.Reason) && evt.Message != "" {
					failures = append(failures, kcommon.Failure{
						Text:      evt.Message,
						Sensitive: []kcommon.Sensitive{},
					})
				}
			} else if containerStatus.State.Waiting.Reason == "CrashLoopBackOff" && containerStatus.LastTerminationState.Terminated != nil {
				// This represents container that is in CrashLoopBackOff state due to conditions such as OOMKilled
				failures = append(failures, kcommon.Failure{
					Text:      fmt.Sprintf("the last termination reason is %s container=%s pod=%s", containerStatus.LastTerminationState.Terminated.Reason, containerStatus.Name, name),
					Sensitive: []kcommon.Sensitive{},
				})
			} else if isErrorReason(containerStatus.State.Waiting.Reason) && containerStatus.State.Waiting.Message != "" {
				failures = append(failures, kcommon.Failure{
					Text:      containerStatus.State.Waiting.Message,
					Sensitive: []kcommon.Sensitive{},
				})
			}
		} else if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.ExitCode != 0 {
			// when pod is Terminated with error
			if !containerStatus.Ready && statusPhase == "Failed" {
				failures = append(failures, kcommon.Failure{
					Text:      fmt.Sprintf("termination reason is %s exitcode=%d container=%s pod=%s", containerStatus.State.Terminated.Reason, containerStatus.State.Terminated.ExitCode, containerStatus.Name, name),
					Sensitive: []kcommon.Sensitive{},
				})
			}
		} else {
			// when pod is Running but its ReadinessProbe fails
			if !containerStatus.Ready && statusPhase == "Running" {
				// parse the event log and append details
				evt, err := util.FetchLatestEvent(a.Context, a.Client, namespace, name)
				if err != nil || evt == nil {
					continue
				}
				if evt.Reason == "Unhealthy" && evt.Message != "" {
					failures = append(failures, kcommon.Failure{
						Text:      evt.Message,
						Sensitive: []kcommon.Sensitive{},
					})
				}
			}
		}
	}

	return failures
}

func isErrorReason(reason string) bool {
	failureReasons := []string{
		"CrashLoopBackOff", "ImagePullBackOff", "CreateContainerConfigError", "PreCreateHookError", "CreateContainerError",
		"PreStartHookError", "RunContainerError", "ImageInspectError", "ErrImagePull", "ErrImageNeverPull", "InvalidImageName",
	}

	for _, r := range failureReasons {
		if r == reason {
			return true
		}
	}
	return false
}

func isEvtErrorReason(reason string) bool {
	failureReasons := []string{
		"FailedCreatePodSandBox", "FailedMount",
	}

	for _, r := range failureReasons {
		if r == reason {
			return true
		}
	}
	return false
}

func podEventWarning(podName string, event prom.Event) common.Warning {
	return common.Warning{
		Text: fmt.Sprintf("Pod %s has %s event at %s %s(%s) count %d", podName, event.Type, event.TimeStamps, event.Reason, event.Message, event.Count),
		Sensitive: []kcommon.Sensitive{
			{
				Unmasked: podName,
				Masked:   util.MaskString(podName),
			},
		},
	}
}

func getContainerLogs(ctx context.Context, client *kubernetes.Client, pod *v1.Pod, container *v1.Container, containerStatus *v1.ContainerStatus) ([]string, error) {
	containerId := strings.TrimPrefix(strings.TrimPrefix(containerStatus.ContainerID, "docker://"), "containerd://")
	reason := ""
	if containerStatus.State.Waiting != nil {
		reason = containerStatus.State.Waiting.Reason
	} else if containerStatus.State.Terminated != nil {
		reason = containerStatus.State.Terminated.Reason
	}

	logs := make([]string, 0)
	if reason != "Completed" && containerId != "" {
		podLogOpts := v1.PodLogOptions{}
		podLogOpts.Follow = true
		podLogOpts.TailLines = &[]int64{int64(60)}[0]
		podLogOpts.Container = container.Name
		req := client.GetClient().CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
		stream, err := req.Stream(ctx)
		if err != nil {
			klog.Errorf("Error list pod(%s/%s -c %s) log, err: %v, ignore", pod.Namespace, pod.Name, container.Name, err)
		} else {
			defer stream.Close()

			reader := bufio.NewScanner(stream)
			for reader.Scan() {
				line := reader.Text()
				logs = append(logs, line)
			}
		}
	}

	return logs, nil
}

func (p PodAnalyzer) Prompt(result *common.Result) (prompt string) {
	if result == nil || (len(result.Error) == 0 && len(result.Warning) == 0) {
		return
	}

	prompt = `你是一个很有帮助的 Kubernetes 集群故障诊断专家，接下来你需要根据我给出的现象（如果没有有效信息，请直接返回正常）帮忙诊断问题，一定需要使用中文来回答.`

	if len(result.Error) > 0 {
		failureText := ""
		for _, e := range result.Error {
			failureText = failureText + e.Text + "\n"
		}
		prompt += fmt.Sprintf("\n异常信息：%s", failureText)
	}

	if len(result.Warning) > 0 {
		warningText := ""
		for _, e := range result.Warning {
			warningText = warningText + e.Text + "\n"
		}
		prompt += fmt.Sprintf("\n一些 Pod 历史事件（如果认为有帮助，可以使用，或者忽略）：%s", warningText)
	}

	if len(result.Info) > 0 {
		infoText := ""
		for _, e := range result.Info {
			infoText = infoText + e.Text + "\n"
		}
		prompt += fmt.Sprintf("\n一些 Pod 日志信息（如果认为有帮助，可以使用，或者忽略）：%s", infoText)
	}

	prompt += `
请按以下格式给出回答，不超过 512 字:
Healthy: {Yes 或者 No，代表是否有异常}
Error: {在这里解释错误}
Solution: {在这里给出分步骤的解决方案}`

	return
}
