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
	networklinkdown_registry_name = string(basic.ConditionTypeNetworkLinkDown)
)

type network struct {
	bridge *sop.ApiBridge
}

var networkInstance *network = &network{}

func init() {
	nodesop.RegisterSOP(networklinkdown_registry_name, networkInstance)
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
	case networklinkdown_registry_name:
		reason := fmt.Sprintf("aegis detect node %s %s device: %s slave device down count: %d", node, status.Condition, status.ID, status.Value)
		basic.CordonNode(ctx, n.bridge, node, status.Condition, "aegis")

		n.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeNetwork, reason)
		n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
		n.bridge.TicketManager.AdoptTicket(ctx)

		n.bridge.TicketManager.AddWhySRE(ctx, "slave device down")
		n.bridge.TicketManager.DispatchTicketToSRE(ctx)

		err := op.DiagnoseNode(ctx, n.bridge, node, status.Condition, status.ID)
		if err != nil {
			klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
		}
		return nil
	}
	return nil
}
