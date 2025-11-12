package basic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/pkg/ticketmodel"
	"github.com/scitix/aegis/tools"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func DiagnoseNode(ctx context.Context, bridge *sop.ApiBridge, node string, tpe string, params ...string) (bool, []ticketmodel.Diagnose, error) {
	podName := fmt.Sprintf("diagnose-%s", node)
	_, err := bridge.KubeClient.CoreV1().Pods(job_namespace).Get(ctx, podName, metav1.GetOptions{})
	if err == nil {
		// delete old pod
		deletePolicy := metav1.DeletePropagationForeground
		err = bridge.KubeClient.CoreV1().Pods(job_namespace).Delete(ctx, podName, metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
		if err != nil {
			return false, nil, fmt.Errorf("Error delete exists diagnose pod %s: %v", podName, err)
		}
	}

	// wait pod cleanup
	WaitPodCleanup(ctx, bridge, podName)

	// apply check pod
	jobContent, err := os.ReadFile(diagnose_job_file)
	if err != nil {
		return false, nil, fmt.Errorf("Error read diagnose template file: %v", err)
	}

	paramsString := strings.Join(params, " ")

	parameters := map[string]interface{}{
		"registry":   bridge.Registry,
		"repository": bridge.Repository,
		"image":      bridge.OpsImage,
		"pod_name":   podName,
		"node_name":  node,
		"alert_name": bridge.AlertName,
		"type":       tpe,
		"parameters": paramsString,
	}

	yamlContent, err := tools.RenderWorkflowTemplate(string(jobContent), parameters)
	if err != nil {
		return false, nil, fmt.Errorf("Error render diagnose template: %v", err)
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		return false, nil, fmt.Errorf("Error decode diagnose pod content: %v", err)
	}
	pod := obj.(*corev1.Pod)
	pod.OwnerReferences = []metav1.OwnerReference{*bridge.Owner}

	_, err = bridge.KubeClient.CoreV1().Pods(job_namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return false, nil, fmt.Errorf("Error create diagnose pod: %v", err)
	}

	// check job compeleted --> node restart
	ticker := time.NewTicker(time.Duration(10) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, nil, errors.New("context done")
		case <-ticker.C:
			status, _, err := CheckPodStatus(ctx, bridge, podName)
			if err != nil {
				return false, nil, err
			}

			if status == 1 {
				logs, err := GetPodLogs(ctx, bridge, podName)
				if err != nil {
					return true, nil, err
				}

				return true, getDiagnose(logs), nil
			} else if status != 0 {
				return false, nil, nil
			} else {
				// go on next
			}
		}
	}
}

func getDiagnose(logs string) []ticketmodel.Diagnose {
	results := make([]ticketmodel.Diagnose, 0)
	loglines := strings.Split(logs, "\n")
	lines := make([]string, 0)

	for index := 0; index < len(loglines); index++ {
		if strings.HasPrefix(loglines[index], "hint:") {
			lines = append(lines, loglines[index][5:])
		} else if strings.HasPrefix(loglines[index], "cmd:") {
			lines = append(lines, loglines[index][4:])
		} else if strings.HasPrefix(loglines[index], "result:") {
			lines = append(lines, loglines[index][7:])
		} else {
			// next loop
		}
	}

	for index := 0; index < len(lines); index++ {
		t := ticketmodel.Diagnose{
			Hint: lines[index],
		}

		if index+1 < len(lines) {
			t.Cmd = lines[index+1]
			index++
		}

		if index+1 < len(lines) {
			t.Result = lines[index+1]
			index++
		}

		results = append(results, t)
	}

	return results
}
