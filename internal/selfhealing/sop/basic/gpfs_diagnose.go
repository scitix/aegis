package basic

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	sop "github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/tools"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
)

func GetDiagnoseGpfsJobName(node string) string {
	return fmt.Sprintf("diagnose-gpfs-%s", node)
}

func DiagnoseGpfs(ctx context.Context, bridge *sop.ApiBridge, node string, component string) (bool, error) {
	jobName := GetDiagnoseGpfsJobName(node)
	_, err := bridge.KubeClient.BatchV1().Jobs(job_namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err == nil {
		// delete old job
		deletePolicy := metav1.DeletePropagationForeground
		err = bridge.KubeClient.BatchV1().Jobs(job_namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
		if err != nil {
			return false, fmt.Errorf("Error delete exists diagnose job %s: %v", jobName, err)
		}
	}

	// wait job cleanup
	WaitJobCleanup(ctx, bridge, jobName)

	// apply check job
	jobContent, err := ioutil.ReadFile(diagnose_gpfs_job_file)
	if err != nil {
		klog.Errorf("Error read diagnose template file: %v", err)
		return false, err
	}

	parameters := map[string]interface{}{
		"registry":   bridge.Registry,
		"repository": bridge.Repository,
		"job_name":   jobName,
		"node_name":  node,
		"component":  component,
		"alert_name": bridge.AlertName,
	}

	yamlContent, err := tools.RenderWorkflowTemplate(string(jobContent), parameters)
	if err != nil {
		fmt.Errorf("Error render diagnose template: %v", err)
		return false, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		fmt.Errorf("Error decode diagnose job content: %v", err)
		return false, err
	}
	job := obj.(*batchv1.Job)

	_, err = bridge.KubeClient.BatchV1().Jobs(job_namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		fmt.Errorf("Error create diagnose job: %v", err)
		return false, err
	}

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
