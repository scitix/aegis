package analyzer

import (
	"context"
	"fmt"

	"github.com/k8sgpt-ai/k8sgpt/pkg/kubernetes"
	aprom "github.com/scitix/aegis/pkg/prom"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func FetchEvents(
	ctx context.Context,
	enableProm bool,
	prom *aprom.PromAPI,
	client *kubernetes.Client,
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
		case "Node", "PyTorchJob", "Workflow": // 其他资源类型用 GetEvent
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
