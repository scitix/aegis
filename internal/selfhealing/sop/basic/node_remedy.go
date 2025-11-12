package basic

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/go-errors/errors"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/tools"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
)

func RemedyNode(ctx context.Context, bridge *sop.ApiBridge, node string, action RemedyAction) (bool, error) {
	podName := fmt.Sprintf("remedy-%s", node)
	_, err := bridge.KubeClient.CoreV1().Pods(job_namespace).Get(ctx, podName, metav1.GetOptions{})
	if err == nil {
		// delete old pod
		deletePolicy := metav1.DeletePropagationForeground
		err = bridge.KubeClient.CoreV1().Pods(job_namespace).Delete(ctx, podName, metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
		if err != nil {
			return false, fmt.Errorf("Error delete exists remedy pod %s: %v", podName, err)
		}
	}

	// wait pod cleanup
	WaitPodCleanup(ctx, bridge, podName)

	// apply check pod
	jobContent, err := ioutil.ReadFile(remedy_job_file)
	if err != nil {
		klog.Errorf("Error read remedy template file: %v", err)
		return false, err
	}

	parameters := map[string]interface{}{
		"registry":   bridge.Registry,
		"repository": bridge.Repository,
		"image":      bridge.OpsImage,
		"pod_name":   podName,
		"node_name":  node,
		"region":     bridge.Region,
		"cluster":    bridge.ClusterName,
		"action":     action,
	}

	yamlContent, err := tools.RenderWorkflowTemplate(string(jobContent), parameters)
	if err != nil {
		fmt.Errorf("Error render remedy template: %v", err)
		return false, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		fmt.Errorf("Error decode remedy pod content: %v", err)
		return false, err
	}
	pod := obj.(*corev1.Pod)
	pod.OwnerReferences = []metav1.OwnerReference{*bridge.Owner}

	_, err = bridge.KubeClient.CoreV1().Pods(job_namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		fmt.Errorf("Error create remedy job: %v", err)
		return false, err
	}

	// defer func() {
	// 	count := 1
	// 	if bridge.NodeStatus.RepairCount != nil {
	// 		count = *bridge.NodeStatus.RepairCount + 1
	// 	}

	// 	AddNodeLabel(ctx, bridge, node, NodeRepairCountKey, fmt.Sprintf("%d", count), "")
	// }()

	// check pod compeleted --> node restart
	ticker := time.NewTicker(time.Duration(10) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, errors.New("context done")
		case <-ticker.C:
			status, _, err := CheckPodStatus(ctx, bridge, podName)
			if err != nil {
				return false, err
			}

			if status != 0 {
				return status == 1, nil
			}
		}
	}
}
