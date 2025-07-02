package nodeticket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/scitix/aegis/pkg/ticketmodel"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type NodeTicket struct {
	Condition  string                       `yaml:"condition,omitempty"`
	Reason     string                       `yaml:"reason,omitempty"`
	Supervisor string                       `yaml:"supervisor"`
	Status     ticketmodel.TicketStatus     `yaml:"status"`
	Workflows  []ticketmodel.TicketWorkflow `yaml:"workflows,omitempty"`
	CreatedAt  time.Time                    `yaml:"creationTime,omitempty"`
}

const MaxAnnotationSize = 4096 // Kubernetes recommends keeping under 4KB

func MarshalNodeTicket(t *NodeTicket) (string, error) {
	clean := *t

	var filteredWorkflows []ticketmodel.TicketWorkflow
	for _, wf := range clean.Workflows {
		filteredWorkflows = append(filteredWorkflows, ticketmodel.TicketWorkflow{
			Action: wf.Action,
			Status: wf.Status,
		})
	}
	clean.Workflows = filteredWorkflows

	b, err := yaml.Marshal(clean)
	if err != nil {
		return "", err
	}
	if len(b) > MaxAnnotationSize {
		return "", fmt.Errorf("NodeTicket too large for annotation: %d bytes", len(b))
	}
	return string(b), nil
}

func UnmarshalNodeTicket(s string) (*NodeTicket, error) {
	var t NodeTicket
	if err := yaml.Unmarshal([]byte(s), &t); err != nil {
		return nil, err
	}
	// Optional: basic validation
	// if t.Condition == "" && len(t.Workflows) == 0 {
	// 	return nil, fmt.Errorf("invalid ticket: missing condition and workflows")
	// }
	return &t, nil
}

const TicketAnnotationKey = "aegis.io/ticketing"

func ReadNodeTicketFromAnnotation(node *corev1.Node) (*NodeTicket, error) {
	s, ok := node.Annotations[TicketAnnotationKey]
	if !ok || s == "" {
		return nil, nil // Not found is not an error
	}
	ticket, err := UnmarshalNodeTicket(s)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal node ticket: %w", err)
	}
	return ticket, nil
}

func WriteNodeTicketToAnnotation(ctx context.Context, client kubernetes.Interface, node *corev1.Node, ticket *NodeTicket) error {
	raw, err := MarshalNodeTicket(ticket)
	if err != nil {
		return fmt.Errorf("marshal ticket failed: %w", err)
	}

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				TicketAnnotationKey: raw,
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal patch failed: %w", err)
	}

	_, err = client.CoreV1().Nodes().Patch(ctx, node.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("patch failed: %w", err)
	}
	return nil
}
