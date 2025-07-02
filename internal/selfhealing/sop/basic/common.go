package basic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func CheckJobStatus(ctx context.Context, bridge *sop.ApiBridge, jobName string) (int, error) {
	maxErrCount := 3
	errCount := 0
	for errCount < maxErrCount {
		job, err := bridge.KubeClient.BatchV1().Jobs(job_namespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.Errorf("Job %s/%s not found.", job_namespace, jobName)
				return 0, errors.New("Not Found")
			}

			klog.Warningf("Get job %s/%s error: %v, the %d try", job_namespace, jobName, err, errCount)
			errCount++
		} else {
			conditions := job.Status.Conditions
			for _, c := range conditions {
				if (c.Type == batch.JobComplete || c.Type == batch.JobFailed) && c.Status == v1.ConditionTrue {
					if c.Type == batch.JobComplete {
						return 1, nil
					} else {
						return -1, nil
					}
				}
			}

			return 0, nil
		}
	}

	return 0, errors.New("exceed max error count, exit")
}

func CheckPodStatus(ctx context.Context, bridge *sop.ApiBridge, podName string) (int, int, error) {
	maxErrCount := 3
	errCount := 0
	for errCount < maxErrCount {
		pod, err := bridge.KubeClient.CoreV1().Pods(job_namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.Errorf("Job %s/%s not found.", job_namespace, podName)
				return 0, 0, errors.New("Not Found")
			}

			klog.Warningf("Get pod %s/%s error: %v, the %d try", job_namespace, podName, err, errCount)
			errCount++
		} else {
			phase := pod.Status.Phase
			switch phase {
			case corev1.PodPending:
				fallthrough
			case corev1.PodRunning:
				fallthrough
			case corev1.PodUnknown:
				return 0, 0, nil
			case corev1.PodSucceeded:
				return 1, 0, nil
			case corev1.PodFailed:
				containerstatuses := pod.Status.ContainerStatuses
				if len(containerstatuses) > 0 {
					return -1, int(pod.Status.ContainerStatuses[0].State.Terminated.ExitCode), nil
				} else {
					return 0, 0, fmt.Errorf("%s: %s", pod.Status.Message, pod.Status.Reason)
				}
			}
		}
	}

	return 0, 0, errors.New("exceed max error count, exit")
}

func CheckPodReady(ctx context.Context, bridge *sop.ApiBridge, pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}

	conditions := pod.Status.Conditions
	for _, c := range conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}

	return false
}

// TODO: more elegant
func WaitJobCleanup(ctx context.Context, bridge *sop.ApiBridge, jobName string) {
	ticker := time.NewTicker(time.Duration(3) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := bridge.KubeClient.BatchV1().Jobs(job_namespace).Get(ctx, jobName, metav1.GetOptions{})
			if err != nil && apierrors.IsNotFound(err) {
				return
			}
		}
	}
}

func WaitPodCleanup(ctx context.Context, bridge *sop.ApiBridge, podName string) {
	ticker := time.NewTicker(time.Duration(3) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := bridge.KubeClient.CoreV1().Pods(job_namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil && apierrors.IsNotFound(err) {
				return
			}
		}
	}
}

func isCreatedByDaemonSet(pod *v1.Pod) bool {
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

func CheckNodeHasUserPod(ctx context.Context, bridge *sop.ApiBridge, node string) bool {
	podList, err := bridge.KubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", node),
	})
	if err != nil {
		klog.Errorf("Error list node %s pod list: %s", node, err)
		return true
	}

	list := make([]string, 0)
	fmt.Printf("Running Pods on node %s (excluding DaemonSet):\n", node)
	for _, pod := range podList.Items {
		// 只选 Running 状态的 Pod
		if pod.Status.Phase != v1.PodRunning {
			continue
		}

		// 过滤 DaemonSet 创建的 Pod
		if isCreatedByDaemonSet(&pod) {
			continue
		}

		list = append(list, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))

		fmt.Printf("- %s/%s\n", pod.Namespace, pod.Name)
	}

	return len(list) > 0
}

// wait user pod completed
// step 1: check gpu pod in running
// step 2: check user pod in running
func WaitNodeCriticalPodCompeleted(ctx context.Context, bridge *sop.ApiBridge, node string, canceled WaitCancelFunc) error {
	ticker := time.NewTicker(time.Duration(2) * time.Hour)
	defer ticker.Stop()

	maxErrCount := 100
	errCount := 0

	for errCount < maxErrCount {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if canceled(ctx) {
				return nil
			}

			timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			statuses, err := bridge.PromClient.GetNodeGpuStatuses(timeoutCtx, node)
			if err != nil {
				klog.Warningf("Get node gpu status error: %v, the %d try", err, errCount)
				errCount++
			}

			existPod := false
			if len(statuses) > 0 {
				klog.V(4).Infof("Gpu status: %v", statuses)
				for _, status := range statuses {
					if status.PodNamespace != "" {
						existPod = true
						break
					}
				}
			}

			if !existPod {
				klog.Infof("node in free state")
				return nil
			}
		}
	}
	return errors.New("exceed max error count, exit")
}

func CheckNodeLabelExists(ctx context.Context, node *corev1.Node, key, value string) bool {
	if node == nil {
		return false
	}

	for k, v := range node.Labels {
		if key == k && value == v {
			return true
		}
	}

	return false
}

func CheckNodeLabelKeyExists(ctx context.Context, node *corev1.Node, key string) bool {
	if node == nil {
		return false
	}

	_, ok := node.Labels[key]
	return ok
}

func CheckNodeIsMasterNode(ctx context.Context, bridge *sop.ApiBridge, nodeName string) bool {
	node, err := bridge.KubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Error get node %s: %s", nodeName, node)
		return false
	}

	var contolplanelabels []string = []string{
		"node-role.kubernetes.io/controlplane",
		"node-role.kubernetes.io/control-plane",
	}

	for key := range node.Labels {
		for _, label := range contolplanelabels {
			if label == key {
				return true
			}
		}
	}

	return false
}

func CheckNodeReady(ctx context.Context, bridge *sop.ApiBridge, nodeName string) bool {
	if nodeName == "" {
		return false
	}

	node, err := bridge.KubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get node %s: %v", nodeName, err)
		return false
	}

	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}
