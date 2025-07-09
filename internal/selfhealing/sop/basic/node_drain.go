package basic

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/drain"
)

// step 1: check node in cordon state
// step 2: drain node via ucp, if failed, try step 3
// step 3: drain node in cluster
func DrainNode(ctx context.Context, bridge *sop.ApiBridge, node, reason, remark string) error {
	n, err := bridge.KubeClient.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if n.Spec.Unschedulable == false {
		klog.Errorf("node %s cannot drain in schedulable state.", node)
		return errors.New("cannot drain in schedulable state")
	}

	helper := &drain.Helper{
		Ctx:                 ctx,
		Client:              bridge.KubeClient,
		Force:               true,
		GracePeriodSeconds:  10,
		IgnoreAllDaemonSets: true,
		Out:                 os.Stdout,
		ErrOut:              os.Stdout,
		DeleteEmptyDirData:  true,
		Timeout:             time.Duration(120) * time.Second,
	}

	return drain.RunNodeDrain(helper, node)
}
