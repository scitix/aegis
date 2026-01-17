package basic

import (
	"context"
	"encoding/json"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// AddNodeTaint 添加节点污点
func AddNodeTaint(ctx context.Context, bridge *sop.ApiBridge, node string, taint v1.Taint, reason string) error {
	n, err := bridge.KubeClient.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get node %s, %v", node, err)
		return err
	}

	// 检查污点是否已存在
	for _, existingTaint := range n.Spec.Taints {
		if existingTaint.Key == taint.Key && existingTaint.Effect == taint.Effect {
			klog.V(4).Infof("node %s taint %s=%s:%s already exists, give up.", node, taint.Key, taint.Value, taint.Effect)
			return nil
		}
	}

	// 添加新的污点
	newTaints := append(n.Spec.Taints, taint)
	patch := []map[string]interface{}{
		{
			"op":    "replace",
			"path":  "/spec/taints",
			"value": newTaints,
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		klog.Errorf("failed to marshal taint patch: %v", err)
		return err
	}

	_, err = bridge.KubeClient.CoreV1().Nodes().Patch(ctx, node, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("failed to add taint to node %s: %v", node, err)
		return err
	}

	klog.Infof("successfully added taint %s=%s:%s to node %s", taint.Key, taint.Value, taint.Effect, node)
	return nil
}

// RemoveNodeTaint 移除节点污点
func RemoveNodeTaint(ctx context.Context, bridge *sop.ApiBridge, node string, taintKey string, reason string) error {
	n, err := bridge.KubeClient.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get node %s, %v", node, err)
		return err
	}

	// 查找并移除匹配的污点
	var remainingTaints []v1.Taint
	found := false
	for _, taint := range n.Spec.Taints {
		if taint.Key == taintKey {
			found = true
			continue // 跳过要移除的污点
		}
		remainingTaints = append(remainingTaints, taint)
	}

	if !found {
		klog.V(4).Infof("node %s taint with key %s not found, give up.", node, taintKey)
		return nil
	}

	// 更新污点列表
	patch := []map[string]interface{}{
		{
			"op":    "replace",
			"path":  "/spec/taints",
			"value": remainingTaints,
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		klog.Errorf("failed to marshal taint removal patch: %v", err)
		return err
	}

	_, err = bridge.KubeClient.CoreV1().Nodes().Patch(ctx, node, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("failed to remove taint from node %s: %v", node, err)
		return err
	}

	klog.Infof("successfully removed taint with key %s from node %s", taintKey, node)
	return nil
}
