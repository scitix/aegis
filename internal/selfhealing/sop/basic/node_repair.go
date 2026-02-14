package basic

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/go-errors/errors"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/tools"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
)

func RepairNode(ctx context.Context, bridge *sop.ApiBridge, node string) (bool, error) {
	jobName := fmt.Sprintf("repair-%s", node)
	_, err := bridge.KubeClient.BatchV1().Jobs(job_namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err == nil {
		// delete old job
		deletePolicy := metav1.DeletePropagationForeground
		err = bridge.KubeClient.BatchV1().Jobs(job_namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
		if err != nil {
			return false, fmt.Errorf("Error delete exists repair job %s: %v", jobName, err)
		}
	}

	// wait job cleanup
	WaitJobCleanup(ctx, bridge, jobName)

	// apply check job
	jobContent, err := ioutil.ReadFile(repair_job_file)
	if err != nil {
		klog.Errorf("Error read repair template file: %v", err)
		return false, err
	}

	parameters := map[string]interface{}{
		"image":     bridge.OpsImage,
		"job_name":  jobName,
		"node_name": node,
		"region":    bridge.Region,
		"cluster":   bridge.ClusterName,
	}

	yamlContent, err := tools.RenderWorkflowTemplate(string(jobContent), parameters)
	if err != nil {
		fmt.Errorf("Error render repair template: %v", err)
		return false, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		fmt.Errorf("Error decode repair job content: %v", err)
		return false, err
	}
	job := obj.(*batchv1.Job)
	job.OwnerReferences = []metav1.OwnerReference{*bridge.Owner}

	_, err = bridge.KubeClient.BatchV1().Jobs(job_namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		fmt.Errorf("Error create repair job: %v", err)
		return false, err
	}

	// defer func() {
	// 	count := 1
	// 	if bridge.NodeStatus.RepairCount != nil {
	// 		count = *bridge.NodeStatus.RepairCount + 1
	// 	}

	// 	AddNodeLabel(ctx, bridge, node, NodeRepairCountKey, fmt.Sprintf("%d", count), "")
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
