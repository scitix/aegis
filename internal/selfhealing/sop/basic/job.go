package basic

import (
	"context"
	"fmt"
	"strings"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
)

func GetJobLogs(ctx context.Context, bridge *sop.ApiBridge, job string) (string, error) {
	pods, err := bridge.KubeClient.CoreV1().Pods(job_namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", job),
	})
	if err != nil {
		return "", fmt.Errorf("error list job %s pods: %s", job, err)
	}

	errs := make([]error, 0)
	logs := make([]string, 0)
	for _, pod := range pods.Items {
		log, err := GetPodLogs(ctx, bridge, pod.Name)
		if err != nil {
			errs = append(errs, err)
		} else {
			logs = append(logs, log)
		}
	}

	return strings.Join(logs, "\n"), errors.NewAggregate(errs)
}
