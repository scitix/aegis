package basic

import (
	"context"
	"encoding/json"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// step 1: check node status in cluster, if cordon, return; if not, go to step 2
// step 2: cordon node via ucp, if failed, try step 3
// step 3: cordon node in cluster
func CordonNode(ctx context.Context, bridge *sop.ApiBridge, node, reason, remark string) error {
	n, err := bridge.KubeClient.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if n.Spec.Unschedulable == true {
		klog.V(4).Infof("node %s already cordon, give up.", node)
	} else {
		patch := []PatchStringValue{{
			Op:    "replace",
			Path:  "/spec/unschedulable",
			Value: true,
		}}

		patchString, _ := json.Marshal(patch)
		_, err = bridge.KubeClient.CoreV1().Nodes().Patch(ctx, node, types.JSONPatchType, patchString, metav1.PatchOptions{})
		if err != nil {
			return err
		}
	}

	// if bridge.NodeStatus.CordonReason == nil || *bridge.NodeStatus.CordonReason != reason {
	// 	return AddNodeLabel(ctx, bridge, node, NodeCordonReasonKey, reason, reason)
	// }

	return nil
}
