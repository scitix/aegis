package nodeticket

import (
	"context"
	"fmt"

	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

// Description Manager Interface

func (m *NodeTicketManager) GetRootCauseDescription(ctx context.Context) (*ticketmodel.TicketCause, error) {
	if m.ticket == nil {
		klog.Infof("[ticket] GetRootCauseDescription called but ticket is nil (no-op in NodeTicketManager)")
		return nil, ticketmodel.TicketNotFoundErr
	}

	klog.Infof("[ticket] GetRootCauseDescription called, returning createdAt timestamp and condition (no-op in NodeTicketManager)")
	return &ticketmodel.TicketCause{
		Timestamps: m.ticket.CreatedAt,
		Condition:  m.ticket.Condition,
	}, nil
}

func (m *NodeTicketManager) AddRootCauseDescription(ctx context.Context, cause string, condition interface{}) (int, error) {
	klog.Infof("[ticket] AddRootCauseDescription called with cause: %s, condition: %+v (no-op in NodeTicketManager)", cause, condition)
	return 0, nil
}

func (m *NodeTicketManager) AddOrUpdateRootCauseDescription(ctx context.Context, cause string, condition interface{}) (bool, error) {
	klog.Infof("[ticket] AddOrUpdateRootCauseDescription called with cause: %s, condition: %+v (no-op in NodeTicketManager)", cause, condition)
	return false, nil
}

func (m *NodeTicketManager) GetActionCount(ctx context.Context, action ticketmodel.TicketWorkflowAction) (int, error) {
	klog.Infof("[ticket] GetActionCount called for action: %s (always returns 0 as no-op in NodeTicketManager)", action)
	return 0, nil
}

func (m *NodeTicketManager) AddConclusion(ctx context.Context, conclusion string) error {
	klog.Infof("[ticket] AddConclusion called with content: %s (no-op in NodeTicketManager)", conclusion)
	return nil
}

func (m *NodeTicketManager) AddDiagnosis(ctx context.Context, diagnosis []ticketmodel.Diagnose) error {
	klog.Infof("[ticket] AddDiagnosis called with %d diagnosis items (no-op in NodeTicketManager)", len(diagnosis))
	return nil
}

func (m *NodeTicketManager) AddWhySRE(ctx context.Context, whySRE string) error {
	klog.Infof("[ticket] AddWhySRE called with reason: %s (no-op in NodeTicketManager)", whySRE)
	return nil
}

func (m *NodeTicketManager) GetWorkflows(ctx context.Context) ([]ticketmodel.TicketWorkflow, error) {
	if m.ticket == nil {
		return nil, ticketmodel.TicketNotFoundErr
	}
	return m.ticket.Workflows, nil
}

func (m *NodeTicketManager) GetLastWorkflow(ctx context.Context) (*ticketmodel.TicketWorkflow, error) {
	if m.ticket == nil {
		return nil, ticketmodel.TicketNotFoundErr
	}
	if len(m.ticket.Workflows) == 0 {
		return nil, nil
	}
	return &m.ticket.Workflows[len(m.ticket.Workflows)-1], nil
}

func (m *NodeTicketManager) AddWorkflow(ctx context.Context, action ticketmodel.TicketWorkflowAction, status ticketmodel.TicketWorkflowStatus, message *string) error {
	if m.ticket == nil {
		return ticketmodel.TicketNotFoundErr
	}

	m.ticket.Workflows = append(m.ticket.Workflows, ticketmodel.TicketWorkflow{
		Action: action,
		Status: status,
	})

	return m.save(m.ticket)
}

func (m *NodeTicketManager) UpdateWorkflow(ctx context.Context, action ticketmodel.TicketWorkflowAction, status ticketmodel.TicketWorkflowStatus, message *string) error {
	if m.ticket == nil {
		return ticketmodel.TicketNotFoundErr
	}

	l := len(m.ticket.Workflows)
	if l == 0 || m.ticket.Workflows[l-1].Action != action {
		return fmt.Errorf("action %s not found", action)
	}

	m.ticket.Workflows[l-1] = ticketmodel.TicketWorkflow{
		Action: action,
		Status: status,
	}

	return m.save(m.ticket)
}


func (m *NodeTicketManager) AddShutdownDescription(ctx context.Context, status ticketmodel.TicketWorkflowStatus, message *string) error {
	return nil
}

func (m *NodeTicketManager) UpdateShutdownDescription(ctx context.Context, status ticketmodel.TicketWorkflowStatus, message *string) error {
	return nil
}