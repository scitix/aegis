package basic

import (
	"context"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	"k8s.io/klog/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func IsDaemonSetExists(ctx context.Context, bridge *sop.ApiBridge, namespace, name string) bool {
	_, err := bridge.KubeClient.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		klog.Infof("failed to get dasemonset %s/%s: %s", namespace, name, err)
		return false
	}

	return true
}
