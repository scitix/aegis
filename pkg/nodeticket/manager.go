package nodeticket

import (
	"context"
	"fmt"
	"time"

	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type NodeTicketManager struct {
	client kubernetes.Interface
	node   *corev1.Node
	user   string
	ticket *NodeTicket
}

func NewNodeTicketManager(ctx context.Context, args *ticketmodel.TicketManagerArgs) (ticketmodel.TicketManagerInterface, error) {
	if args.Node == nil {
		return nil, fmt.Errorf("node object is nil in TicketManagerArgs")
	}
	if args.Client == nil {
		return nil, fmt.Errorf("kube client is nil in TicketManagerArgs")
	}
	ticket, err := ReadNodeTicketFromAnnotation(args.Node)
	if err != nil {
		return nil, fmt.Errorf("failed to read node ticket from annotation: %w", err)
	}

	return &NodeTicketManager{
		client: args.Client,
		node:   args.Node,
		user:   args.User,
		ticket: ticket,
	}, nil
}

func (m *NodeTicketManager) Reset(ctx context.Context) error {
	t, err := ReadNodeTicketFromAnnotation(m.node)
	if err != nil {
		return err
	}
	m.ticket = t
	return nil
}

func (m *NodeTicketManager) CanDealWithTicket(ctx context.Context) bool {
	return m.ticket == nil || m.ticket.Supervisor == m.user
}

func (m *NodeTicketManager) CheckTicketExists(ctx context.Context) bool {
	return m.ticket != nil
}

func (m *NodeTicketManager) CheckTicketSupervisor(ctx context.Context, user string) bool {
	if m.ticket == nil {
		return false
	}
	return m.ticket.Supervisor == user
}

func (m *NodeTicketManager) CreateTicket(ctx context.Context, status *prom.AegisNodeStatus, hardwareType string, customTitle ...string) error {
	title := fmt.Sprintf("aegis detect node %s %s, type: %s %s, reason: %s",
		m.node.Name, status.Condition, status.Type, status.ID, status.Msg)

	if len(customTitle) > 0 && customTitle[0] != "" {
		title = customTitle[0]
	}

	newTicket := &NodeTicket{
		Condition:  status.Condition,
		Reason:     title,
		Supervisor: m.user,
		Status:     ticketmodel.TicketStatusCreated,
		CreatedAt:  time.Now(),
	}

	if m.ticket == nil ||
		m.ticket.Status == ticketmodel.TicketStatusResolved ||
		m.ticket.Status == ticketmodel.TicketStatusClosed ||
		m.ticket.Condition != newTicket.Condition {
		return m.save(newTicket)
	}

	if m.ticket.Supervisor == newTicket.Supervisor {
		return ticketmodel.TicketAlreadyExistErr
	}

	return m.save(newTicket)
}

func (m *NodeTicketManager) CreateComponentTicket(ctx context.Context, title, model, component string) error {
	newTicket := &NodeTicket{
		Condition:  fmt.Sprintf("component/%s", component),
		Reason:     title,
		Supervisor: "aegis",
		Status:     ticketmodel.TicketStatusCreated,
		CreatedAt:  time.Now(),
	}

	if m.ticket == nil ||
		m.ticket.Status == ticketmodel.TicketStatusResolved ||
		m.ticket.Status == ticketmodel.TicketStatusClosed ||
		m.ticket.Condition != newTicket.Condition {
		return m.save(newTicket)
	}

	if m.ticket.Reason == newTicket.Reason && m.ticket.Supervisor == newTicket.Supervisor {
		return ticketmodel.TicketAlreadyExistErr
	}

	return m.save(newTicket)
}

func (m *NodeTicketManager) AdoptTicket(ctx context.Context) error {
	if m.ticket == nil {
		klog.Warningf("[ticket] AdoptTicket called but ticket is nil (no-op)")
		return ticketmodel.TicketNotFoundErr
	}
	klog.Infof("[ticket] AdoptTicket called (no-op in NodeTicketManager)")
	return nil
}

func (m *NodeTicketManager) DispatchTicket(ctx context.Context, user string) error {
	if m.ticket == nil {
		klog.Warningf("[ticket] DispatchTicket called but ticket is nil (no-op)")
		return ticketmodel.TicketNotFoundErr
	}
	klog.Infof("[ticket] DispatchTicket called (no-op in NodeTicketManager)")
	return nil
}

func (m *NodeTicketManager) DispatchTicketToSRE(ctx context.Context) error {
	return m.DispatchTicket(ctx, "")
}

func (m *NodeTicketManager) ResolveTicket(ctx context.Context, answer, operation string) error {
	if m.ticket == nil {
		klog.Warningf("[ticket] ResolveTicket called but ticket is nil (no-op)")
		return nil
	}
	klog.Infof("[ticket] ResolveTicket called with answer: %s, operation: %s", answer, operation)
	m.ticket.Status = ticketmodel.TicketStatusResolved
	return m.save(m.ticket)
}

func (m *NodeTicketManager) CloseTicket(ctx context.Context) error {
	if m.ticket == nil {
		klog.Warningf("[ticket] CloseTicket called but ticket is nil (no-op)")
		return nil
	}
	klog.Infof("[ticket] CloseTicket called")
	m.ticket.Status = ticketmodel.TicketStatusClosed
	err := m.save(m.ticket)
	if err != nil {
		klog.Errorf("[ticket] CloseTicket failed to save ticket: %v", err)
	} else {
		klog.Infof("[ticket] CloseTicket successful, clearing in-memory ticket")
		m.ticket = nil
	}
	return err
}

func (t *NodeTicketManager) DeleteTicket(ctx context.Context) error {
	return t.CloseTicket(ctx)
}

func (m *NodeTicketManager) IsFrequentIssue(ctx context.Context, size, frequency int) (bool, error) {
	klog.Infof("[ticket] IsFrequentIssue called with size=%d, frequency=%d (always returns false in NodeTicketManager)", size, frequency)
	return false, nil
}

func (m *NodeTicketManager) save(t *NodeTicket) error {
	klog.Infof("[ticket] saving ticket to node %s, condition=%s", m.node.Name, t.Condition)

	err := WriteNodeTicketToAnnotation(context.Background(), m.client, m.node, t)
	if err != nil {
		klog.Errorf("[ticket] failed to save ticket to annotation: %v", err)
	} else {
		klog.Infof("[ticket] successfully saved ticket to annotation")
		m.ticket = t
	}
	return err
}