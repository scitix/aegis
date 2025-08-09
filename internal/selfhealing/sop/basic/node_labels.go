package basic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

func AddNodeLabel(ctx context.Context, bridge *sop.ApiBridge, node, key, value, reason string) error {
	label := fmt.Sprintf("%s=%s", key, value)
	n, err := bridge.KubeClient.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get node %s, %v", node, err)
		return err
	}

	if CheckNodeLabelExists(ctx, n, key, value) {
		klog.V(4).Infof("node %s label %s exists, give up.", node, label)
		return nil
	}

	patch := []PatchStringValue{{
		Op:    "add",
		Path:  fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(key, "/", "~1")),
		Value: value,
	}}

	if CheckNodeLabelKeyExists(ctx, n, key) {
		patch = []PatchStringValue{{
			Op:    "replace",
			Path:  fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(key, "/", "~1")),
			Value: value,
		}}
	}

	patchString, _ := json.Marshal(patch)
	_, err = bridge.KubeClient.CoreV1().Nodes().Patch(ctx, node, types.JSONPatchType, patchString, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("patch node %s label %s failed, %v", node, label, err)
	}

	return err
}

func DeleteNodeLabel(ctx context.Context, bridge *sop.ApiBridge, node, key, value, reason string) error {
	label := fmt.Sprintf("%s=%s", key, value)
	n, err := bridge.KubeClient.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get node %s failed, %v", node, err)
		return err
	}

	if !CheckNodeLabelExists(ctx, n, key, value) {
		klog.V(4).Infof("node %s label %s not exists, give up.", node, label)
		return nil
	}

	patch := []PatchStringValue{{
		Op:   "remove",
		Path: fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(key, "/", "~1")),
	}}

	patchString, _ := json.Marshal(patch)
	_, err = bridge.KubeClient.CoreV1().Nodes().Patch(ctx, node, types.JSONPatchType, patchString, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("patch node %s label %s failed, %v", node, label, err)
	}

	return err
}
