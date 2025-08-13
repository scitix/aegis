package noneticket

import (
	"context"

	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

// Description Manager Interface

func (m *NoneTicketManager) GetRootCauseDescription(ctx context.Context) (*ticketmodel.TicketCause, error) {
	klog.Infof("[ticket] AddRootCauseDescription called (no-op in NoneTicketManager)")
	return nil, nil
}

func (m *NoneTicketManager) AddRootCauseDescription(ctx context.Context, cause string, condition interface{}) (int, error) {
	klog.Infof("[ticket] AddRootCauseDescription called with cause: %s, condition: %+v (no-op in NoneTicketManager)", cause, condition)
	return 0, nil
}

func (m *NoneTicketManager) AddOrUpdateRootCauseDescription(ctx context.Context, cause string, condition interface{}) (bool, error) {
	klog.Infof("[ticket] AddOrUpdateRootCauseDescription called with cause: %s, condition: %+v (no-op in NoneTicketManager)", cause, condition)
	return false, nil
}

func (m *NoneTicketManager) GetActionCount(ctx context.Context, action ticketmodel.TicketWorkflowAction) (int, error) {
	klog.Infof("[ticket] GetActionCount called for action: %s (always returns 0 as no-op in NoneTicketManager)", action)
	return 0, nil
}

func (m *NoneTicketManager) AddConclusion(ctx context.Context, conclusion string) error {
	klog.Infof("[ticket] AddConclusion called with content: %s (no-op in NoneTicketManager)", conclusion)
	return nil
}

func (m *NoneTicketManager) AddDiagnosis(ctx context.Context, diagnosis []ticketmodel.Diagnose) error {
	klog.Infof("[ticket] AddDiagnosis called with %d diagnosis items (no-op in NoneTicketManager)", len(diagnosis))
	return nil
}

func (m *NoneTicketManager) AddWhySRE(ctx context.Context, whySRE string) error {
	klog.Infof("[ticket] AddWhySRE called with reason: %s (no-op in NoneTicketManager)", whySRE)
	return nil
}

func (m *NoneTicketManager) GetWorkflows(ctx context.Context) ([]ticketmodel.TicketWorkflow, error) {
	klog.Infof("[ticket] GetWorkflows called (no-op in NoneTicketManager)")
	return nil, nil
}

func (m *NoneTicketManager) GetLastWorkflow(ctx context.Context) (*ticketmodel.TicketWorkflow, error) {
	klog.Infof("[ticket] GetLastWorkflow called (no-op in NoneTicketManager)")
	return nil, nil
}

func (m *NoneTicketManager) AddWorkflow(ctx context.Context, action ticketmodel.TicketWorkflowAction, status ticketmodel.TicketWorkflowStatus, message *string) error {
	klog.Infof("[ticket] AddWorkflow called with action: %s (no-op in NoneTicketManager)", action)
	return nil
}

func (m *NoneTicketManager) UpdateWorkflow(ctx context.Context, action ticketmodel.TicketWorkflowAction, status ticketmodel.TicketWorkflowStatus, message *string) error {
	klog.Infof("[ticket] UpdateWorkflow called with action: %s (no-op in NoneTicketManager)", action)
	return nil
}

func (m *NoneTicketManager) AddShutdownDescription(ctx context.Context, status ticketmodel.TicketWorkflowStatus, message *string) error {
	return nil
}

func (m *NoneTicketManager) UpdateShutdownDescription(ctx context.Context, status ticketmodel.TicketWorkflowStatus, message *string) error {
	return nil
}