package basic

import (
	"context"
	"fmt"
	"time"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	EventReason        string
	EventMessageFormat string
)

const (
	EventReasonEviction           EventReason = "TaintManagerEviction"
	EventReasonPotentialInfluence EventReason = "AegisPotentialInfluence"

	EvictionMessageFormat           EventMessageFormat = "Aegis Marking for deletion Pod %s/%s because of %s"
	PotentailInfluenceMessageFormat EventMessageFormat = "Aegis potential influence Pod %s/%s because of %s"
)

var reasonMessageMap = map[EventReason]EventMessageFormat{
	EventReasonEviction:           EvictionMessageFormat,
	EventReasonPotentialInfluence: PotentailInfluenceMessageFormat,
}

func FireEventForNodePod(ctx context.Context, bridge *sop.ApiBridge, node string, reason EventReason, triggerReason string) error {
	pods, err := GetPodInNode(ctx, bridge, node)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		// not a daemonset pod
		controllerRef := metav1.GetControllerOf(pod)
		if controllerRef != nil && controllerRef.Kind == "DaemonSet" {
			continue
		}

		bridge.EventRecorder.Event(pod, v1.EventTypeWarning, string(reason), fmt.Sprintf(string(reasonMessageMap[reason]), pod.Namespace, pod.Name, triggerReason))
	}

	time.Sleep(10 * time.Second)

	return nil
}
