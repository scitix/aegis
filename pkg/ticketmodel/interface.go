package ticketmodel

import (
	"context"

	"github.com/scitix/aegis/pkg/prom"
)

type TicketManagerInterface interface {
	// Core Ticket Manage
	Reset(ctx context.Context) error
	CanDealWithTicket(ctx context.Context) bool
	CheckTicketExists(ctx context.Context) bool
	CheckTicketSupervisor(ctx context.Context, user string) bool
	CreateTicket(ctx context.Context, status *prom.AegisNodeStatus, hardwareType string, customTitle ...string) error
	CreateComponentTicket(ctx context.Context, title, model, component string) error
	AdoptTicket(ctx context.Context) error
	DispatchTicket(ctx context.Context, user string) error
	DispatchTicketToSRE(ctx context.Context, opts ...string) error
	ResolveTicket(ctx context.Context, answer, operation string) error
	CloseTicket(ctx context.Context) error
	DeleteTicket(ctx context.Context) error
	IsFrequentIssue(ctx context.Context, size, frequency int) (bool, error)

	// Description Manager
	AddRootCauseDescription(ctx context.Context, title string, condition interface{}) (int, error)
	AddOrUpdateRootCauseDescription(ctx context.Context, cause string, condition interface{}) (bool, error)
	AddConclusion(ctx context.Context, conclusion string) error
	AddDiagnosis(ctx context.Context, diagnosis []Diagnose) error
	AddWhySRE(ctx context.Context, whySRE string) error
	GetRootCauseDescription(ctx context.Context) (*TicketCause, error)
	GetActionCount(ctx context.Context, action TicketWorkflowAction) (int, error)

	// Workflow operate
	GetWorkflows(ctx context.Context) ([]TicketWorkflow, error)
	GetLastWorkflow(ctx context.Context) (*TicketWorkflow, error)
	AddWorkflow(ctx context.Context, action TicketWorkflowAction, status TicketWorkflowStatus, message *string) error
	UpdateWorkflow(ctx context.Context, action TicketWorkflowAction, status TicketWorkflowStatus, message *string) error

	AddShutdownDescription(ctx context.Context, status TicketWorkflowStatus, message *string) error
	UpdateShutdownDescription(ctx context.Context, status TicketWorkflowStatus, message *string) error
}
