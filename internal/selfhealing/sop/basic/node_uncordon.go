package basic

import (
	"context"
	"encoding/json"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// step 1: check node status in cluster, if schedulable, return; if not, go to step 2
// step 2: uncordon node in cluster
func UncordonNode(ctx context.Context, bridge *sop.ApiBridge, node, remark string) error {
	n, err := bridge.KubeClient.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if n.Spec.Unschedulable == false {
		klog.V(4).Infof("node %s already schedulable, give up.", node)
	} else {
		patch := []PatchStringValue{{
			Op:    "replace",
			Path:  "/spec/unschedulable",
			Value: false,
		}}

		patchString, _ := json.Marshal(patch)
		_, err = bridge.KubeClient.CoreV1().Nodes().Patch(ctx, node, types.JSONPatchType, patchString, metav1.PatchOptions{})
		if err != nil {
			klog.Errorf("uncordon node %s failed: %v", node, err)
			return err
		}
	}

	// if bridge.NodeStatus.CordonReason != nil {
	// 	DeleteNodeLabel(ctx, bridge, node, NodeCordonReasonKey, *bridge.NodeStatus.CordonReason, remark)
	// }

	// if bridge.NodeStatus.RebootCount != nil {
	// 	DeleteNodeLabel(ctx, bridge, node, NodeRebootCountKey, fmt.Sprintf("%d", *bridge.NodeStatus.RebootCount), remark)
	// }

	// if bridge.NodeStatus.RepairCount != nil {
	// 	DeleteNodeLabel(ctx, bridge, node, NodeRepairCountKey, fmt.Sprintf("%d", *bridge.NodeStatus.RepairCount), remark)
	// }

	return nil
}
