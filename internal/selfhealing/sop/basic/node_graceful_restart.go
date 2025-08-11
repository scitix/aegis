package basic

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/tools"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
)

// check no system pod running
// drain node
// try to restart node
func NodeGracefulRestart(ctx context.Context, bridge *sop.ApiBridge, node, reason, remark string, cancel WaitCancelFunc) (bool, error) {
	// check no system pod running
	has := CheckNodeHasUserPod(ctx, bridge, node)
	if has {
		return false, nil
	}

	if cancel(ctx) {
		return false, nil
	}

	err := FireEventForNodePod(ctx, bridge, node, EventReasonEviction, reason)
	if err != nil {
		klog.Warningf("fail to send evivtion event: %s", err)
	}

	// drain node
	err = DrainNode(ctx, bridge, node, reason, remark)
	if err != nil {
		return false, err
	}

	// restart node, 20min
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, time.Minute*time.Duration(30))
	defer timeoutCancel()
	return restartNode(timeoutCtx, bridge, node)
}

func restartNode(ctx context.Context, bridge *sop.ApiBridge, node string) (bool, error) {
	jobName := fmt.Sprintf("reboot-%s", node)
	_, err := bridge.KubeClient.BatchV1().Jobs(job_namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err == nil {
		// delete old job
		deletePolicy := metav1.DeletePropagationForeground
		err = bridge.KubeClient.BatchV1().Jobs(job_namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
		if err != nil {
			return false, fmt.Errorf("Error delete exists reboot job %s: %v", jobName, err)
		}
	}

	// wait job cleanup
	WaitJobCleanup(ctx, bridge, jobName)

	// apply restart job
	jobContent, err := ioutil.ReadFile(reboot_job_file)
	if err != nil {
		fmt.Errorf("Error read reboot template file: %v", err)
		return false, err
	}

	parameters := map[string]interface{}{
		"registry":   bridge.Registry,
		"repository": bridge.Repository,
		"image":      bridge.OpsImage,
		"job_name":   jobName,
		"node_name":  node,
	}

	yamlContent, err := tools.RenderWorkflowTemplate(string(jobContent), parameters)
	if err != nil {
		klog.Errorf("Error render reboot template: %v", err)
		return false, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		klog.Errorf("Error decode reboot job content: %v", err)
		return false, err
	}
	job := obj.(*batchv1.Job)

	_, err = bridge.KubeClient.BatchV1().Jobs(job_namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("Error create reboot job: %v", err)
		return false, err
	}

	// defer func() {
	// 	count := 1
	// 	if bridge.NodeStatus.RebootCount != nil {
	// 		count = *bridge.NodeStatus.RebootCount + 1
	// 	}

	// 	AddNodeLabel(ctx, bridge, node, NodeRebootCountKey, fmt.Sprintf("%d", count), "")
	// }()

	// check job compeleted --> node restart
	ticker := time.NewTicker(time.Duration(10) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, errors.New("context done")
		case <-ticker.C:
			status, err := CheckJobStatus(ctx, bridge, jobName)
			if err != nil {
				return false, err
			}

			if status != 0 {
				return status == 1, nil
			}
		}
	}
}
