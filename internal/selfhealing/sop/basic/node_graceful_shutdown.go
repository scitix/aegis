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

// wait gpu pod running completed
// drain node
// try to shutdown node
func NodeGracefulShutdown(ctx context.Context, bridge *sop.ApiBridge, node, reason, remark string, cancel WaitCancelFunc) (bool, error) {
	if bridge.AggressiveLevel < 2 {
		return false, fmt.Errorf("cannot shutdown node because of AggressiveLevel: %d which should > 1", bridge.AggressiveLevel)
	}

	// wait cirtical pod running completed, 4d
	dayCtx, dayCancel := context.WithTimeout(ctx, time.Hour*time.Duration(4*24))
	defer dayCancel()
	err := WaitNodeCriticalPodCompeleted(dayCtx, bridge, node, cancel)
	if err != nil {
		return false, err
	}

	if cancel(ctx) {
		return false, nil
	}

	err = FireEventForNodePod(ctx, bridge, node, EventReasonEviction, reason)
	if err != nil {
		klog.Warningf("fail to send evivtion event: %s", err)
	}

	// drain node
	err = DrainNode(ctx, bridge, node, reason, remark)
	if err != nil {
		return false, err
	}

	// shutdown node, 20min
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, time.Minute*time.Duration(30))
	defer timeoutCancel()
	return shutdownNode(timeoutCtx, bridge, node)
}

func shutdownNode(ctx context.Context, bridge *sop.ApiBridge, node string) (bool, error) {
	jobName := fmt.Sprintf("shutdown-%s", node)
	_, err := bridge.KubeClient.BatchV1().Jobs(job_namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err == nil {
		// delete old job
		deletePolicy := metav1.DeletePropagationForeground
		err = bridge.KubeClient.BatchV1().Jobs(job_namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
		if err != nil {
			return false, fmt.Errorf("Error delete exists shutdown job %s: %v", jobName, err)
		}
	}

	// wait job cleanup
	WaitJobCleanup(ctx, bridge, jobName)

	// apply shutdown job
	jobContent, err := ioutil.ReadFile(shutdown_job_file)
	if err != nil {
		klog.Errorf("Error read shutdown template file: %v", err)
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
		klog.Errorf("Error render shutdown template: %v", err)
		return false, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		klog.Errorf("Error decode shutdown job content: %v", err)
		return false, err
	}
	job := obj.(*batchv1.Job)

	_, err = bridge.KubeClient.BatchV1().Jobs(job_namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("Error create shutdown job: %v", err)
		return false, err
	}

	// check node shutdown
	ticker := time.NewTicker(time.Duration(10) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, errors.New("context done")
		case <-ticker.C:
			ready := CheckNodeReady(ctx, bridge, node)
			if !ready {
				return true, nil
			}
		}
	}
}
