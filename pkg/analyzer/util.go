package analyzer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	kcommon "github.com/k8sgpt-ai/k8sgpt/pkg/common"
	kkubernetes "github.com/k8sgpt-ai/k8sgpt/pkg/kubernetes"
	aprom "github.com/scitix/aegis/pkg/prom"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func FetchEvents(
	ctx context.Context,
	enableProm bool,
	prom *aprom.PromAPI,
	client *kkubernetes.Client,
	objectKind, namespace, name, eventType, timeRange string,
) (any, error) {
	if enableProm {
		if prom == nil {
			return nil, fmt.Errorf("EnableProm=true but prom is nil")
		}
		if timeRange == "" {
			timeRange = "7d"
		}
		switch objectKind {
		case "Pod":
			return prom.GetEventWithRange(ctx, "Pod", namespace, name, eventType, timeRange)
		case "Node", "PyTorchJob", "Workflow": // GetEvent
			return prom.GetEvent(ctx, objectKind, namespace, name, eventType)
		default:
			return nil, fmt.Errorf("unsupported objectKind for Prometheus: %s", objectKind)
		}
	}

	// K8s events fallback
	eventList, err := client.GetClient().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s", name, objectKind),
	})
	if err != nil {
		return nil, err
	}

	var filtered []corev1.Event
	for _, evt := range eventList.Items {
		if eventType == "" || evt.Type == eventType {
			filtered = append(filtered, evt)
		}
	}
	return filtered, nil
}

func FetchNodeFailures(
	ctx context.Context,
	enableProm bool,
	prom *aprom.PromAPI,
	client *kkubernetes.Client,
	nodeName string,
) ([]kcommon.Failure, error) {
	var failures []kcommon.Failure

	if enableProm {
		if prom == nil {
			return nil, fmt.Errorf("EnableProm=true but prom is nil")
		}
		nodeConditions, err := prom.GetNodeStatuses(ctx, nodeName, "")
		if err != nil {
			return nil, err
		}
		for _, cond := range nodeConditions {
			failures = append(failures, nodeStatusFailure(nodeName, cond))
		}
	} else {
		node, err := client.GetClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		for _, cond := range node.Status.Conditions {
			switch cond.Type {
			case corev1.NodeReady:
				if cond.Status == corev1.ConditionTrue {
					continue
				}
			case corev1.NodeConditionType("EtcdIsVoter"):
				continue
			default:
				if cond.Status == corev1.ConditionFalse {
					continue
				}
			}
			failures = append(failures, nodeStatusFailureLegacy(node.Name, cond))
		}
	}
	return failures, nil
}


func CheckPodStatus(ctx context.Context, client kubernetes.Interface, namespace, podName string) (int, int, error) {
	maxErrCount := 3
	errCount := 0
	for errCount < maxErrCount {
		pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.Errorf("Pod %s/%s not found.", namespace, podName)
				return 0, 0, errors.New("Not Found")
			}

			klog.Warningf("Get pod %s/%s error: %v, try %d", namespace, podName, err, errCount)
			errCount++
			continue
		}

		phase := pod.Status.Phase
		switch phase {
		case corev1.PodPending, corev1.PodRunning, corev1.PodUnknown:
			return 0, 0, nil
		case corev1.PodSucceeded:
			return 1, 0, nil
		case corev1.PodFailed:
			containerstatuses := pod.Status.ContainerStatuses
			if len(containerstatuses) > 0 {
				return -1, int(containerstatuses[0].State.Terminated.ExitCode), nil
			}
			return 0, 0, fmt.Errorf("%s: %s", pod.Status.Message, pod.Status.Reason)
		}
	}

	return 0, 0, errors.New("exceed max error count, exit")
}

func GetPodLogs(ctx context.Context, client kubernetes.Interface, namespace, podName string) (string, error) {
	podLogOpts := corev1.PodLogOptions{}
	req := client.CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("error opening log stream: %s", err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("error copying pod logs: %s", err)
	}
	return buf.String(), nil
}
