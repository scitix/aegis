package noneticket

import (
	"context"

	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type NoneTicketManager struct {
	client kubernetes.Interface
	node   *corev1.Node
}

func NewNoneTicketManager(ctx context.Context, args *ticketmodel.TicketManagerArgs) (ticketmodel.TicketManagerInterface, error) {
	return &NoneTicketManager{
		client: args.Client,
		node:   args.Node,
	}, nil
}

func (m *NoneTicketManager) Reset(ctx context.Context) error {
	return nil
}

func (m *NoneTicketManager) CanDealWithTicket(ctx context.Context) bool {
	return true
}

func (m *NoneTicketManager) GetTicketCondition(ctx context.Context) string {
	return ""
}

func (m *NoneTicketManager) CheckTicketExists(ctx context.Context) bool {
	return false
}

func (m *NoneTicketManager) CheckTicketSupervisor(ctx context.Context, user string) bool {
	return true
}

func (m *NoneTicketManager) CreateTicket(ctx context.Context, status *prom.AegisNodeStatus, hardwareType string, customTitle ...string) error {
	return nil
}

func (m *NoneTicketManager) CreateComponentTicket(ctx context.Context, title, model, component string) error {
	return nil
}

func (m *NoneTicketManager) AdoptTicket(ctx context.Context) error {
	return nil
}

func (m *NoneTicketManager) DispatchTicket(ctx context.Context, user string) error {
	return nil
}

func (m *NoneTicketManager) DispatchTicketToSRE(ctx context.Context, opts ...string) error {
	return nil
}

func (m *NoneTicketManager) ResolveTicket(ctx context.Context, answer, operation string) error {
	return nil
}

func (m *NoneTicketManager) CloseTicket(ctx context.Context) error {
	return nil
}

func (t *NoneTicketManager) DeleteTicket(ctx context.Context) error {
	return t.CloseTicket(ctx)
}

func (m *NoneTicketManager) IsFrequentIssue(ctx context.Context, size, frequency int) (bool, error) {
	return false, nil
}