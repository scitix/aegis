package basic

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-errors/errors"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/tools"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
)

func HealthCheckNode(ctx context.Context, bridge *sop.ApiBridge, node string) (bool, HardwareType, ConditionType, error) {
	podName := fmt.Sprintf("healthcheck-%s", node)
	_, err := bridge.KubeClient.CoreV1().Pods(job_namespace).Get(ctx, podName, metav1.GetOptions{})
	if err == nil {
		// delete old pod
		deletePolicy := metav1.DeletePropagationForeground
		err = bridge.KubeClient.CoreV1().Pods(job_namespace).Delete(ctx, podName, metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
		if err != nil {
			return false, HardwareTypeNone, ConditionTypeNull, fmt.Errorf("Error delete exists healthchech pod %s: %v", podName, err)
		}
	}

	// wait pod cleanup
	WaitPodCleanup(ctx, bridge, podName)

	// apply check pod
	jobContent, err := os.ReadFile(healthcheck_job_file)
	if err != nil {
		klog.Errorf("Error read healthcheck template file: %v", err)
		return false, HardwareTypeNone, ConditionTypeNull, err
	}

	parameters := map[string]interface{}{
		"registry":   bridge.Registry,
		"repository": bridge.Repository,
		"image":      bridge.OpsImage,
		"pod_name":   podName,
		"node_name":  node,
		"region":     bridge.Region,
		"cluster":    bridge.ClusterName,
	}

	yamlContent, err := tools.RenderWorkflowTemplate(string(jobContent), parameters)
	if err != nil {
		klog.Errorf("Error render healthcheck template: %v", err)
		return false, HardwareTypeNone, ConditionTypeNull, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		klog.Errorf("Error decode healthcheck pod content: %v", err)
		return false, HardwareTypeNone, ConditionTypeNull, err
	}
	pod := obj.(*corev1.Pod)

	_, err = bridge.KubeClient.CoreV1().Pods(job_namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("Error create healthcheck job: %v", err)
		return false, HardwareTypeNone, ConditionTypeNull, err
	}

	// check pod compeleted --> node restart
	ticker := time.NewTicker(time.Duration(10) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, HardwareTypeNone, ConditionTypeNull, errors.New("context done")
		case <-ticker.C:
			status, exitcode, err := CheckPodStatus(ctx, bridge, podName)
			hardwareType, conditionType := getHardwareTypeByExitCode(exitcode)
			if err != nil {
				return false, hardwareType, conditionType, err
			}

			if status != 0 {
				return status == 1, hardwareType, conditionType, nil
			}
		}
	}
}

func getHardwareTypeByExitCode(exitcode int) (HardwareType, ConditionType) {
	switch exitcode {
	case 0:
		return HardwareTypeNone, ConditionTypeNull
	case 1:
		return HardwareTypeIB, ConditionTypeNull
	case 2:
		return HardwareTypeGpu, ConditionTypeNull
	case 21:
		return HardwareTypeGpu, ConditionTypeGpuRowRemappingFailure
	case 22:
		return HardwareTypeGpu, ConditionTypeGpuAggSramUncorrectable
	case 23:
		return HardwareTypeGpu, ConditionTypeGpuCheckFailed
	case 3:
		return HardwareTypeGpfs, ConditionTypeNull
	case 4:
		return HardwareTypeCpu, ConditionTypeNull
	case 5:
		return HardwareTypeMemory, ConditionTypeNull
	case 6:
		return HardwareTypeDisk, ConditionTypeNull
	case 7:
		return HardwareTypeNetwork, ConditionTypeNull
	default:
		return HardwareTypeUnknown, ConditionTypeNull
	}
}
