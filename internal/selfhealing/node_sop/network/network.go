package network

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/internal/selfhealing/sop/op"
	"github.com/scitix/aegis/pkg/prom"
)

const (
	networklinkfrequentdown_registry_name = string(basic.ConditionTypeNetworkLinkFrequentDown)
	networklinktoomanydown_registry_name  = string(basic.ConditionTypeNetworkLinkTooManyDown)
)

type network struct {
	bridge *sop.ApiBridge
}

var networkInstance *network = &network{}

func init() {
	nodesop.RegisterSOP(networklinkfrequentdown_registry_name, networkInstance)
	nodesop.RegisterSOP(networklinktoomanydown_registry_name, networkInstance)
}

func (n *network) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	networkInstance.bridge = bridge
	return nil
}

func (n *network) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

// try to cordon node and create a ticket
func (n *network) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	switch status.Condition {
	case networklinktoomanydown_registry_name:
		reason := fmt.Sprintf("aegis detect node %s %s device: %s too many down, count: %d", node, status.Condition, status.ID, status.Value)
		basic.CordonNode(ctx, n.bridge, node, status.Condition, "aegis")

		n.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeNetwork, reason)
		n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
		n.bridge.TicketManager.AdoptTicket(ctx)

		n.bridge.TicketManager.AddWhySRE(ctx, "over 100 times network link down")
		n.bridge.TicketManager.DispatchTicketToSRE(ctx)

		err := op.DiagnoseNode(ctx, n.bridge, node, status.Condition, status.ID)
		if err != nil {
			klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
		}
		return nil
	case networklinkfrequentdown_registry_name:
		reason := fmt.Sprintf("aegis detect node %s %s device: %s frequent: %d/24h", node, status.Condition, status.ID, status.Value)
		basic.CordonNode(ctx, n.bridge, node, status.Condition, "aegis")

		n.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeNetwork, reason)
		n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
		n.bridge.TicketManager.AdoptTicket(ctx)
		n.bridge.TicketManager.AddWhySRE(ctx, "network link frequent down in 24h")
		n.bridge.TicketManager.DispatchTicketToSRE(ctx)

		err := op.DiagnoseNode(ctx, n.bridge, node, status.Condition, status.ID)
		if err != nil {
			klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
		}
		return nil

	}
	return nil
}
