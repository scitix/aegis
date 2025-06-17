package analyzer

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/k8sgpt-ai/k8sgpt/pkg/analyzer"
	kcommon "github.com/k8sgpt-ai/k8sgpt/pkg/common"
	"github.com/k8sgpt-ai/k8sgpt/pkg/kubernetes"
	"github.com/k8sgpt-ai/k8sgpt/pkg/util"
	ai "github.com/scitix/aegis/pkg/ai"
	"github.com/scitix/aegis/pkg/analyzer/common"
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/tools"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
)

type NodeAnalyzer struct {
	prometheus *prom.PromAPI
}

func NewNodeAnalyzer(enableProm bool) NodeAnalyzer {
	var promAPI *prom.PromAPI
	if enableProm {
		promAPI = prom.GetPromAPI()
	} else {
		promAPI = nil
	}
	return NodeAnalyzer{
		prometheus: promAPI,
	}
}

func (n NodeAnalyzer) Analyze(a common.Analyzer) (*common.Result, error) {
	kind := "Node"

	analyzer.AnalyzerErrorsMetric.DeletePartialMatch(map[string]string{
		"analyzer_name": kind,
	})

	// selector, err := labels.Parse(a.LabelSelector)
	// if err != nil {
	// 	return nil, err
	// }

	// hostname, found := selector.RequiresExactMatch("kubernetes.io/hostname")
	// if !found {
	// 	return nil, fmt.Errorf("label selector must have hostname")
	// }
	hostname := a.Name

	// check node exists
	node, err := a.Client.GetClient().CoreV1().Nodes().Get(a.Context, hostname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// node condition
	failures, err := FetchNodeFailures(a.Context, a.EnableProm, n.prometheus, a.Client, node.Name)
	if err != nil {
		klog.Warningf("fetch node failures failed: %v", err)
	}

	var warnings []common.Warning
	// event
	rawEvents, err := FetchEvents(a.Context, a.EnableProm, n.prometheus, a.Client, "Node", "", node.Name, "Warning", "")
	if err != nil {
		klog.Warningf("fetch node events failed: %v", err)
	} else {
		if a.EnableProm {
			for _, event := range rawEvents.([]prom.Event) {
				warnings = append(warnings, nodeEventWarning(node.Name, event))
			}
		} else {
			for _, event := range rawEvents.([]corev1.Event) {
				warnings = append(warnings, nodeEventWarningLegacy(node.Name, event))
			}
		}
	}

	var infos []common.Info
	// Start collector pod
	logs, err := StartCollector(a.Context, a.Client, node, a.CollectorImage, a.Owner.GetNamespace(), a.Owner)
	if err != nil {
		klog.Warningf("Start collector failed: %v", err)
	} else {
		infos = append(infos, logs...)
	}

	if len(failures) > 0 {
		analyzer.AnalyzerErrorsMetric.WithLabelValues(kind, node.Name, "").Set(float64(len(failures)))
	}

	return &common.Result{
		Result: kcommon.Result{
			Kind:  kind,
			Name:  node.Name,
			Error: failures,
		},
		Warning: warnings,
		Info:    infos,
	}, nil
}

func nodeStatusFailure(nodeName string, status prom.AegisNodeStatus) kcommon.Failure {
	return kcommon.Failure{
		Text: fmt.Sprintf("condition %s type=%s id=%s value=%d", status.Condition, status.Type, status.ID, status.Value),
		Sensitive: []kcommon.Sensitive{
			{
				Unmasked: nodeName,
				Masked:   util.MaskString(nodeName),
			},
		},
	}
}

func nodeStatusFailureLegacy(nodeName string, cond corev1.NodeCondition) kcommon.Failure {
	return kcommon.Failure{
		Text: fmt.Sprintf("node %s has condition %s (status=%s, reason=%s): %s", nodeName, cond.Type, cond.Status, cond.Reason, cond.Message),
		Sensitive: []kcommon.Sensitive{
			{
				Unmasked: nodeName,
				Masked:   util.MaskString(nodeName),
			},
		},
	}
}

func nodeEventWarning(nodeName string, event prom.Event) common.Warning {
	return common.Warning{
		Text: fmt.Sprintf("has %s event at %s %s(%s) count %d", event.Type, event.TimeStamps, event.Reason, event.Message, event.Count),
		Sensitive: []kcommon.Sensitive{
			{
				Unmasked: nodeName,
				Masked:   util.MaskString(nodeName),
			},
		},
	}
}

func nodeEventWarningLegacy(nodeName string, event corev1.Event) common.Warning {
	timestamp := event.LastTimestamp.Time
	if timestamp.IsZero() {
		timestamp = event.EventTime.Time
	}
	return common.Warning{
		Text: fmt.Sprintf("has %s event at %s %s(%s) count %d", event.Type, timestamp.Format(time.RFC3339), event.Reason, event.Message, event.Count),
		Sensitive: []kcommon.Sensitive{
			{
				Unmasked: nodeName,
				Masked:   util.MaskString(nodeName),
			},
		},
	}
}

func nodeLogInfo(nodeName string, logs string) []common.Info {
	var infos []common.Info
	lines := strings.Split(logs, "\n")
	var currentSection string
	var content []string

	flush := func() {
		if currentSection != "" {
			text := fmt.Sprintf("[%s]\n%s", currentSection, strings.Join(content, "\n"))
			infos = append(infos, common.Info{
				Text: text,
				Sensitive: []kcommon.Sensitive{
					{
						Unmasked: nodeName,
						Masked:   util.MaskString(nodeName),
					},
				},
			})
		}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			currentSection = strings.Trim(line, "[]")
			content = []string{}
		} else if strings.HasPrefix(line, "- ") {
			content = append(content, line)
		}
	}

	flush()

	return infos
}

func StartCollector(
	ctx context.Context,
	client *kubernetes.Client,
	node *corev1.Node,
	image string,
	namespace string,
	owner metav1.Object,
) ([]common.Info, error) {
	collector_pod_yaml := "/collector/collector_node.yaml"
	tplContent, err := os.ReadFile(collector_pod_yaml)
	if err != nil {
		return nil, fmt.Errorf("error reading collector template: %v", err)
	}

	parameters := map[string]interface{}{
		"generateName":   fmt.Sprintf("collector-%s-", node.Name),
		"namespace":      namespace,
		"collectorImage": image,
		"node_name":      node.Name,
	}

	yamlContent, err := tools.RenderWorkflowTemplate(string(tplContent), parameters)
	if err != nil {
		return nil, fmt.Errorf("error rendering collector template: %v", err)
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("error decoding collector pod: %v", err)
	}

	pod := obj.(*corev1.Pod)
	pod.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(owner, schema.GroupVersionKind{
			Group:   "aegis.io",
			Version: "v1alpha1",
			Kind:    "AegisDiagnosis",
		}),
	}

	created, err := client.GetClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("error creating collector pod: %v", err)
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context canceled before collector finished")
		case <-ticker.C:
			status, _, err := CheckPodStatus(ctx, client.GetClient(), namespace, created.Name)
			if err != nil {
				return nil, err
			}
			if status == 1 {
				logs, err := GetPodLogs(ctx, client.GetClient(), namespace, created.Name)
				if err != nil {
					return nil, fmt.Errorf("get collector logs error: %v", err)
				}
				return nodeLogInfo(node.Name, logs), nil
			} else if status != 0 {
				return nil, fmt.Errorf("collector pod status failed")
			}
		}
	}
}

func (NodeAnalyzer) Prompt(result *common.Result) (prompt string) {
	if result == nil || (len(result.Error) == 0 && len(result.Warning) == 0) {
		return ""
	}

	errorInfo := ""
	for _, e := range result.Error {
		errorInfo += e.Text + "\n"
	}

	eventInfo := ""
	for _, e := range result.Warning {
		eventInfo += e.Text + "\n"
	}

	logInfo := ""
	for _, e := range result.Info {
		logInfo += e.Text + "\n"
	}

	data := ai.PromptData{
		ErrorInfo: errorInfo,
		EventInfo: eventInfo,
		LogInfo:   logInfo,
	}

	prompt, err := ai.GetRenderedPrompt("Node", data)
	if err != nil {
		return err.Error()
	}
	return prompt
}
