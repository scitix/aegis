package basic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_labels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

func GetPodLogs(ctx context.Context, bridge *sop.ApiBridge, pod string) (string, error) {
	podLogOpts := v1.PodLogOptions{}
	req := bridge.KubeClient.CoreV1().Pods(job_namespace).GetLogs(pod, &podLogOpts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("error in opening stream: %s", err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("error copy from podLogs to buf: %s", err)
	}
	return buf.String(), nil
}

func DeletePodInNodeWithTargetLabel(ctx context.Context, bridge *sop.ApiBridge, node string, labels map[string]string, repair bool) error {
	pods, err := GetPodInNodeWithTargetLabel(ctx, bridge, node, labels)
	if err != nil {
		return err
	}

	if len(pods) != 1 {
		return fmt.Errorf("Unexpected pod count. expected 1, got: %d", len(pods))
	}

	err = bridge.KubeClient.CoreV1().Pods(pods[0].Namespace).Delete(ctx, pods[0].Name, metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("error delete pod: %s", err)
		return err
	} else {
		klog.Infof("succeed delete pod")
	}

	return nil
}

func IsPodInNodeWithTargetLabelReady(ctx context.Context, bridge *sop.ApiBridge, node string, labels map[string]string) bool {
	pods, err := GetPodInNodeWithTargetLabel(ctx, bridge, node, labels)
	if err != nil {
		klog.Warningf("Get Pod in node(%s) with label(%v) ready failed：%s", node, labels, err)
		return false
	} else if len(pods) != 1 {
		klog.Warningf("Unexpected pod count. expected 1, got: %d", len(pods))
		return false
	}

	return CheckPodReady(ctx, bridge, pods[0])
}

func WaitPodInNodeWithTargetLabelReady(ctx context.Context, bridge *sop.ApiBridge, node string, labels map[string]string) error {
	ticker := time.NewTicker(time.Duration(10) * time.Second)
	defer ticker.Stop()

	maxErr := 10
	curErr := 0

	for true {
		select {
		case <-ctx.Done():
			return errors.New("context done")
		case <-ticker.C:
			pods, err := GetPodInNodeWithTargetLabel(ctx, bridge, node, labels)
			if err != nil {
				curErr++
				klog.Warningf("Get Pod in node(%s) with label(%v) ready failed：%s", node, labels, err)
			} else if len(pods) != 1 {
				curErr++
				klog.Warningf("Unexpected pod count. expected 1, got: %d", len(pods))
			}

			if CheckPodReady(ctx, bridge, pods[0]) {
				return nil
			}

			if curErr > maxErr {
				return errors.New("too many errors for get pod")
			}
		}
	}

	return errors.New("pod not ready")
}

func GetPodInNodeWithTargetLabel(ctx context.Context, bridge *sop.ApiBridge, node string, labels map[string]string) ([]*v1.Pod, error) {
	if node == "" {
		return nil, errors.New("Empty node name")
	}

	if len(labels) == 0 {
		return nil, errors.New("Empty labels")
	}

	listOptions := metav1.ListOptions{
		LabelSelector: _labels.Set(labels).String(),
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", node),
	}

	pods, err := bridge.KubeClient.CoreV1().Pods(metav1.NamespaceAll).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	results := make([]*v1.Pod, 0)
	for _, pod := range pods.Items {
		klog.Infof("find pod %s/%s in node %s with labels %s", pod.Namespace, pod.Name, node, _labels.Set(labels).String())
		klog.Infof("\tphase: %s", pod.Status.Phase)

		results = append(results, &pod)
	}
	return results, nil
}

func GetPodInNode(ctx context.Context, bridge *sop.ApiBridge, node string) ([]*v1.Pod, error) {
	if node == "" {
		return nil, errors.New("Empty node name")
	}

	listOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", node),
	}

	pods, err := bridge.KubeClient.CoreV1().Pods(metav1.NamespaceAll).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	results := make([]*v1.Pod, 0)
	for _, pod := range pods.Items {
		klog.Infof("find pod %s/%s in node %s", pod.Namespace, pod.Name, node)
		klog.Infof("\tphase: %s", pod.Status.Phase)

		p := pod.DeepCopy()
		results = append(results, p)
	}
	return results, nil
}

func GetTerminatingPodInNode(ctx context.Context, bridge *sop.ApiBridge, node string) ([]*v1.Pod, error) {
	if node == "" {
		return nil, errors.New("Empty node name")
	}

	listOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", node),
	}

	pods, err := bridge.KubeClient.CoreV1().Pods(metav1.NamespaceAll).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	results := make([]*v1.Pod, 0)
    // 过滤出处于 Terminating 的 Pod
    for _, pod := range pods.Items {
        if pod.DeletionTimestamp != nil {
            results = append(results, &pod)
        }
    }
	
	return results, nil
}