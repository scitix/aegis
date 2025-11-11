package analyzer

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/k8sgpt-ai/k8sgpt/pkg/analyzer"
	kcommon "github.com/k8sgpt-ai/k8sgpt/pkg/common"
	"github.com/k8sgpt-ai/k8sgpt/pkg/kubernetes"
	"github.com/k8sgpt-ai/k8sgpt/pkg/util"
	ai "github.com/scitix/aegis/pkg/ai"
	"github.com/scitix/aegis/pkg/analyzer/common"
	"github.com/scitix/aegis/pkg/prom"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type PodAnalyzer struct {
	prometheus *prom.PromAPI
}

func NewPodAnalyzer(prometheus *prom.PromAPI) PodAnalyzer {
	return PodAnalyzer{
		prometheus: prometheus,
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
	// pod event
	rawEvents, err := FetchEvents(a.Context, a.EnableProm, p.prometheus, a.Client, kind, pod.Namespace, pod.Name, "", "7d")
	if err != nil {
		klog.Warningf("fetch pod events failed: %v", err)
	} else {
		if a.EnableProm {
			for _, event := range rawEvents.([]prom.Event) {
				warnings = append(warnings, podEventWarning(pod.Name, event))
			}
		} else {
			for _, event := range rawEvents.([]v1.Event) {
				warnings = append(warnings, podEventWarningLegacy(pod.Name, event))
			}
		}
	}
	

	var infos []common.Info
	enablePodLog := true
	if a.EnablePodLog != nil {
		enablePodLog = *a.EnablePodLog
	}

	if enablePodLog && shouldFetchLog(pod) {
		infos = append(infos, fetchContainerLogs(a.Context, a.Client, pod)...)
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

func shouldFetchLog(pod *v1.Pod) bool {
	statuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
	for _, status := range statuses {
		if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
			return true
		}
		if status.State.Waiting != nil && status.State.Waiting.Reason == "CrashLoopBackOff" {
			return true
		}
	}
	return false
}

func fetchContainerLogs(ctx context.Context, client *kubernetes.Client, pod *v1.Pod) []common.Info {
	var infos []common.Info

	findStatus := func(name string, list []v1.ContainerStatus) *v1.ContainerStatus {
		for _, s := range list {
			if s.Name == name {
				return &s
			}
		}
		return nil
	}

	for _, c := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
		status := findStatus(c.Name, append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...))
		if status == nil {
			continue
		}
		if logs, err := getContainerLogs(ctx, client, pod, &c, status); err == nil && len(logs) > 0 {
			infos = append(infos, common.Info{
				Text: fmt.Sprintf("pod %s container %s logs:\n%s", pod.Name, c.Name, strings.Join(logs, "\n")),
			})
		}
	}

	return infos
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

func podEventWarningLegacy(podName string, event v1.Event) common.Warning {
	timestamp := event.LastTimestamp.Time
	if timestamp.IsZero() {
		timestamp = event.EventTime.Time
	}
	return common.Warning{
		Text: fmt.Sprintf("Pod %s has %s event at %s %s(%s) count %d", podName, event.Type, timestamp.Format(time.RFC3339), event.Reason, event.Message, event.Count),
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
		return ""
	}

	errorInfo := ""
	for _, e := range result.Error {
		errorInfo += e.Text + "\n"
	}

	eventInfo := ""
	for _, e := range result.Warning {
		eventInfo += e.Text + "\n"
	}

	logInfo := ""
	for _, e := range result.Info {
		logInfo += e.Text + "\n"
	}

	data := ai.PromptData{
		ErrorInfo: errorInfo,
		EventInfo: eventInfo,
		LogInfo:   logInfo,
	}

	prompt, err := ai.GetRenderedPrompt("Pod", data)
	if err != nil {
		return err.Error()
	}
	return prompt
}

func (p PodAnalyzer) Summarize(result *common.Result) string {
	if result == nil {
		return "Unable to analyze this Pod. No result data was returned."
	}

	errorInfo := ""
	for _, e := range result.Error {
		errorInfo += e.Text + "\n"
	}

	eventInfo := ""
	for _, e := range result.Warning {
		eventInfo += e.Text + "\n"
	}

	logInfo := ""
	for _, e := range result.Info {
		logInfo += e.Text + "\n"
	}

	if strings.TrimSpace(errorInfo) == "" && strings.TrimSpace(eventInfo) == "" && strings.TrimSpace(logInfo) == "" {
        return "No issues detected. Pod is running and healthy."
    }

	return fmt.Sprintf(
		"Errors:\n%s\nEvents:\n%s\nLogs:\n%s",
		strings.TrimSpace(errorInfo),
		strings.TrimSpace(eventInfo),
		strings.TrimSpace(logInfo),
	)
}
