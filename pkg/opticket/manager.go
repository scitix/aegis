package opticket

import (
	"context"
	"fmt"
	"os"

	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

type OpTicketManager struct {
	region   string
	cluster  string
	orgname  string
	nodename string
	ip       string
	sn       string
	user     string
	ticket   *OpTicket
	u        *OpTicketClient
}

func NewOPTicketManager(ctx context.Context, args *ticketmodel.TicketManagerArgs) (ticketmodel.TicketManagerInterface, error) {
	endpoint := os.Getenv("OP_ENDPOINT")
	if endpoint == "" {
		return nil, fmt.Errorf("ticketing system endpoint not found.")
	}

	u, err := CreateOpTicketClient(endpoint)
	if err != nil {
		return nil, err
	}

	instance, err := u.GetNodeInfo(ctx, args.Region, args.OrgName, args.Ip)
	if err != nil {
		return nil, err
	}
	if instance == nil {
		return nil, fmt.Errorf("error get node info: %s", err)
	}
	klog.Infof("CES Node Instance info: %+v", instance)

	sn := instance.SN

	ticket, err := u.GetNodeFirstUnResovledTicket(ctx, args.Region, sn)
	if err != nil {
		return nil, fmt.Errorf("error get node ticket info: %s", err)
	}
	klog.Infof("Node Ticket info: %+v", ticket)

	return &OpTicketManager{
		region:   args.Region,
		cluster:  args.ClusterName,
		orgname:  args.OrgName,
		nodename: args.NodeName,
		ip:       args.Ip,
		sn:       sn,
		user:     args.User,
		ticket:   ticket,
		u:        u,
	}, nil
}

func (t *OpTicketManager) Reset(ctx context.Context) error {
	ticket, err := t.u.GetNodeFirstUnResovledTicket(ctx, t.region, t.sn)
	if err != nil {
		return fmt.Errorf("error get node ticket info: %s", err)
	}

	klog.V(4).Infof("Node Ticket info: %+v", ticket)

	t.ticket = ticket

	return nil
}

func (t *OpTicketManager) CanDealWithTicket(ctx context.Context) bool {
	return t.ticket == nil || t.ticket.Supervisor == t.user
}

func (t *OpTicketManager) CheckTicketExists(ctx context.Context) bool {
	return t.ticket != nil
}

func (t *OpTicketManager) CheckTicketSupervisor(ctx context.Context, user string) bool {
	if t.ticket == nil {
		return false
	}
	return t.ticket.Supervisor == user
}

func (t *OpTicketManager) CreateTicket(ctx context.Context, status *prom.AegisNodeStatus, hardwareType string, customTitle ...string) error {
	if t.ticket != nil {
		return ticketmodel.TicketAlreadyExistErr
	}

	title := fmt.Sprintf("aegis detect node %s %s, type: %s %s, reason: %s",
		t.nodename, status.Condition, status.Type, status.ID, status.Msg)

	if len(customTitle) > 0 && customTitle[0] != "" {
		title = customTitle[0]
	}

	err := t.u.CreateTicket(ctx, t.region, t.orgname, t.nodename, t.sn, title, "", hardwareType)
	if err != nil {
		return fmt.Errorf("error create ticket: %s", err)
	}

	t.ticket, err = t.u.GetNodeFirstUnResovledTicket(ctx, t.region, t.sn)
	if err != nil {
		return fmt.Errorf("error get ticket: %s", err)
	}

	return nil
}

func (t *OpTicketManager) CreateComponentTicket(ctx context.Context, title, model, component string) error {
	if t.ticket != nil {
		return ticketmodel.TicketAlreadyExistErr
	}

	err := t.u.CreateComponentTicket(ctx, t.region, t.orgname, t.nodename, t.sn, title, "", model, component)
	if err != nil {
		return fmt.Errorf("error create component ticket: %s", err)
	}

	t.ticket, err = t.u.GetNodeFirstUnResovledTicket(ctx, t.region, t.sn)
	if err != nil {
		return fmt.Errorf("error get ticket: %s", err)
	}

	return nil
}

func (t *OpTicketManager) AdoptTicket(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			klog.Errorf("error adopt ticket: %s", err)
		}
	}()

	if t.ticket == nil {
		return ticketmodel.TicketNotFoundErr
	}

	err = t.u.DispatchTicket(ctx, t.ticket.TicketId, TicketSupervisorAegis)
	if err != nil {
		return fmt.Errorf("error dispatch ticket: %s", err)
	}

	err = t.u.AcceptTicket(ctx, t.ticket.TicketId)
	if err != nil {
		return fmt.Errorf("error dispatch ticket: %s", err)
	}

	t.ticket, err = t.u.GetNodeFirstUnResovledTicket(ctx, t.region, t.sn)
	if err != nil {
		return fmt.Errorf("error get ticket: %s", err)
	}
	return nil
}

func (t *OpTicketManager) DispatchTicketToSRE(ctx context.Context) (err error) {
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

	err = t.u.DispatchTicket(ctx, t.ticket.TicketId, GetTicketSupervisorSRE())
	if err != nil {
		return fmt.Errorf("error dispatch ticket: %s", err)
	}

	t.ticket, err = t.u.GetNodeFirstUnResovledTicket(ctx, t.region, t.sn)
	if err != nil {
		return fmt.Errorf("error get ticket: %s", err)
	}
	return nil
}

func (t *OpTicketManager) ResolveTicket(ctx context.Context, answer, operation string) error {
	if t.ticket == nil {
		return nil
	}

	err := t.u.ResolveTicket(ctx, t.ticket.TicketId, answer, operation, t.ticket.IsHardwareIssue)
	if err != nil {
		return fmt.Errorf("error resolve ticket: %s", err)
	}

	t.ticket, err = t.u.GetNodeFirstUnResovledTicket(ctx, t.region, t.sn)
	if err != nil {
		return fmt.Errorf("error get ticket: %s", err)
	}
	return nil
}

func (t *OpTicketManager) CloseTicket(ctx context.Context) error {
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

func (t *OpTicketManager) IsFrequentIssue(ctx context.Context, size, frequency int) (bool, error) {
	tickets, err := t.u.ListNodeTickets(ctx, t.region, t.sn, size)
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
