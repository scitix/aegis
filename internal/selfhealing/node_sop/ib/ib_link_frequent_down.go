package ib

import (
	"context"
	"fmt"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
)

const iblinkfrequentdown_registry_name = string(basic.ConditionTypeIBLinkFrequentDown)

type iblinkfrequentdown struct {
	bridge *sop.ApiBridge
}

var iblinkfrequentdownInstance *iblinkfrequentdown = &iblinkfrequentdown{}

func init() {
	nodesop.RegisterSOP(iblinkfrequentdown_registry_name, iblinkfrequentdownInstance)
	// nodesop.RegisterSOP(iblinkdown_registry_name, ibInstance)
	// nodesop.RegisterSOP(ibreceivederr_registry_name, ibInstance)
	// nodesop.RegisterSOP(ibtransmittederr_registry_name, ibInstance)
}

func (n *iblinkfrequentdown) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	iblinkfrequentdownInstance.bridge = bridge
	return nil
}

func (n *iblinkfrequentdown) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

// try to cordon node and create a ticket
func (n *iblinkfrequentdown) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	switch status.Condition {
	// case ibreceivederr_registry_name:
	// 	fallthrough
	// case ibtransmittederr_registry_name:
	// 	fallthrough
	// case iblinkdown_registry_name:
	// 	reason := fmt.Sprintf("aegis detect node %s %s device: %s value: %d, decide cordon node 6H and wait", node, status.Condition, status.ID, status.Value)
	// 	err := basic.CordonNode(ctx, n.bridge, node, reason, "aegis")
	// 	if err != nil {
	// 		return err
	// 	}

	// 	n.bridge.TicketManager.CreateTicket(ctx, reason, basic.HardwareTypeIB)
	// 	count, _ := n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	// 	n.bridge.TicketManager.AdoptTicket(ctx)

	// 	if count > 3 {
	// 		n.bridge.TicketManager.AddConclusion(ctx, "ib errors over 3 times")
	// 		n.bridge.TicketManager.DispatchTicketToSRE(ctx)
	// 	}
	case iblinkfrequentdown_registry_name:
		reason := fmt.Sprintf("aegis detect node %s %s device: %s frequent: %d", node, status.Condition, status.ID, status.Value)
		err := basic.CordonNode(ctx, n.bridge, node, reason, "aegis")
		if err != nil {
			return err
		}

		n.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeIB, reason)
		n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
		n.bridge.TicketManager.AdoptTicket(ctx)
		n.bridge.TicketManager.AddConclusion(ctx, "ib link frequent down in 24h")
		n.bridge.TicketManager.DispatchTicketToSRE(ctx)
	}

	return nil
}

func (g *iblinkfrequentdown) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}

