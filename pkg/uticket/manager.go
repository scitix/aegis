package uticket

import (
	"context"
	"fmt"
	"os"

	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

type TicketManager struct {
	cluster string
	node    string
	user    string
	ticket  *TicketInfo
	u       *Client
}

func NewTicketManager(ctx context.Context, args *ticketmodel.TicketManagerArgs) (*TicketManager, error) {
	endpoint := os.Getenv("U_ENDPOINT")
	if endpoint == "" {
		return nil, fmt.Errorf("ticketing system endpoint not found.")
	}

	u, err := CreateClient(endpoint)
	if err != nil {
		return nil, err
	}

	ticket, err := u.GetNodeFirstUnResovledTicket(ctx, args.ClusterName, args.NodeName)
	if err != nil {
		return nil, fmt.Errorf("error get node ticket info: %s", err)
	}

	klog.Infof("Node Ticket info: %+v", ticket)

	return &TicketManager{
		cluster: args.ClusterName,
		node:    args.NodeName,
		user:    args.User,
		ticket:  ticket,
		u:       u,
	}, nil
}

func (t *TicketManager) Reset(ctx context.Context) error {
	ticket, err := t.u.GetNodeFirstUnResovledTicket(ctx, t.cluster, t.node)
	if err != nil {
		return fmt.Errorf("error get node ticket info: %s", err)
	}

	klog.V(4).Infof("Node Ticket info: %+v", ticket)

	t.ticket = ticket

	return nil
}

func (t *TicketManager) CanDealWithTicket(ctx context.Context) bool {
	return t.ticket == nil || t.ticket.Supervisor == t.user
}

func (t *TicketManager) GetTicketCondition(ctx context.Context) string {
	if t.ticket == nil {
		return ""
	}
	var description ticketmodel.TicketDescription
	if err := description.Unmarshal([]byte(t.ticket.Description)); err != nil {
		return ""
	}
	if m, ok := description.Cause.Condition.(map[interface{}]interface{}); ok {
		if cond, ok := m["condition"].(string); ok {
			return cond
		}
	}
	return ""
}

func (t *TicketManager) CheckTicketExists(ctx context.Context) bool {
	return t.ticket != nil
}

func (t *TicketManager) CheckTicketSupervisor(ctx context.Context, user string) bool {
	if t.ticket == nil {
		return false
	}
	return t.ticket.Supervisor == user
}

func (t *TicketManager) CreateTicket(ctx context.Context, status *prom.AegisNodeStatus, hardwareType string, customTitle ...string) error {
	if t.ticket != nil {
		return ticketmodel.TicketAlreadyExistErr
	}

	title := fmt.Sprintf("aegis detect node %s %s, type: %s",
		status.Name, status.Condition, status.Type)
	if status.ID != "" {
		title = fmt.Sprintf("%s, id: %s", title, status.ID)
	}

	if status.Msg != "" {
		title = fmt.Sprintf("%s, msg: %s", title, status.Msg)
	}

	if len(customTitle) > 0 && customTitle[0] != "" {
		title = customTitle[0]
	}

	err := t.u.CreateTicket(ctx, t.cluster, t.node, title, "", hardwareType, RangeTypeNode)
	if err != nil {
		return fmt.Errorf("error create ticket: %s", err)
	}

	t.ticket, err = t.u.GetNodeFirstUnResovledTicket(ctx, t.cluster, t.node)
	if err != nil {
		return fmt.Errorf("error get ticket: %s", err)
	}

	return nil
}

func (t *TicketManager) CreateComponentTicket(ctx context.Context, title, model, component string) error {
	if t.ticket != nil {
		return ticketmodel.TicketAlreadyExistErr
	}

	err := t.u.CreateComponentTicket(ctx, t.cluster, t.node, title, "", model, component)
	if err != nil {
		return fmt.Errorf("error create component ticket: %s", err)
	}

	t.ticket, err = t.u.GetNodeFirstUnResovledTicket(ctx, t.cluster, t.node)
	if err != nil {
		return fmt.Errorf("error get ticket: %s", err)
	}

	return nil
}

func (t *TicketManager) AdoptTicket(ctx context.Context) (err error) {
	if t.ticket == nil {
		return ticketmodel.TicketNotFoundErr
	}

	defer func() {
		if err != nil {
			klog.Errorf("error adopt ticket: %s", err)
		}
	}()

	err = t.u.DispatchTicket(ctx, t.ticket.TicketId, TicketSupervisorAegis)
	if err != nil {
		return fmt.Errorf("error dispatch ticket: %s", err)
	}

	err = t.u.AcceptTicket(ctx, t.ticket.TicketId)
	if err != nil {
		return fmt.Errorf("error dispatch ticket: %s", err)
	}

	t.ticket, err = t.u.GetNodeFirstUnResovledTicket(ctx, t.cluster, t.node)
	if err != nil {
		return fmt.Errorf("error get ticket: %s", err)
	}
	return nil
}

func (t *TicketManager) DispatchTicket(ctx context.Context, user string) (err error) {
	defer func() {
		if err != nil {
			klog.Errorf("error dispatch to sre: %s", err)
		}
	}()

	if t.ticket == nil {
		return ticketmodel.TicketNotFoundErr
	}

	if t.ticket.Supervisor != TicketSupervisorAegis {
		return nil
	}

	err = t.u.DispatchTicket(ctx, t.ticket.TicketId, user)
	if err != nil {
		return fmt.Errorf("error dispatch ticket: %s", err)
	}

	t.ticket, err = t.u.GetNodeFirstUnResovledTicket(ctx, t.cluster, t.node)
	if err != nil {
		return fmt.Errorf("error get ticket: %s", err)
	}
	return nil
}

func (t *TicketManager) DispatchTicketToSRE(ctx context.Context, opts ...string) (err error) {
	return t.DispatchTicket(ctx, GetTicketSupervisorSRE())
}

func (t *TicketManager) ResolveTicket(ctx context.Context, answer, operation string) error {
	if t.ticket == nil {
		return nil
	}

	err := t.u.ResolveTicket(ctx, t.ticket.TicketId, t.cluster, t.node, answer, operation, t.ticket.IsHardwareIssue)
	if err != nil {
		return fmt.Errorf("error resolve ticket: %s", err)
	}

	t.ticket, err = t.u.GetNodeFirstUnResovledTicket(ctx, t.cluster, t.node)
	if err != nil {
		return fmt.Errorf("error get ticket: %s", err)
	}
	return nil
}

func (t *TicketManager) CloseTicket(ctx context.Context) error {
	if t.ticket == nil {
		return nil
	}

	err := t.u.CloseTicket(ctx, t.ticket.TicketId)
	if err != nil {
		return fmt.Errorf("error close ticket: %s", err)
	}

	t.ticket = nil
	return nil
}

func (t *TicketManager) DeleteTicket(ctx context.Context) error {
	if t.ticket == nil {
		return nil
	}

	err := t.u.DeleteTicket(ctx, t.ticket.TicketId)
	if err != nil {
		return fmt.Errorf("error delete ticket: %s", err)
	}

	t.ticket = nil
	return nil
}

func (t *TicketManager) IsFrequentIssue(ctx context.Context, size, frequency int) (bool, error) {
	tickets, err := t.u.ListNodeTickets(ctx, t.cluster, t.node, size)
	if err != nil {
		return false, fmt.Errorf("error list latest 10 ticket: %s", err)
	}

	if len(tickets) < frequency+1 {
		return false, nil
	}

	conds := make([]map[interface{}]interface{}, 0)
	for _, ticket := range tickets {
		var description ticketmodel.TicketDescription
		err := description.Unmarshal([]byte(ticket.Description))
		if err != nil {
			klog.Info("error unmarshal ticket %s: %s", ticket.TicketId, err)
			continue
		}

		m, ok := description.Cause.Condition.(map[interface{}]interface{})
		if !ok {
			continue
		}

		conds = append(conds, m)
	}

	condition := conds[0]["condition"].(string)
	tp := conds[0]["type"].(string)
	id := conds[0]["id"].(string)
	count := 0

	for _, m := range conds {
		if m["condition"].(string) == condition &&
			m["type"].(string) == tp &&
			m["id"].(string) == id {
			count++
		}
	}

	return count > frequency, nil
}
