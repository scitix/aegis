package ib

import (
	"context"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const (
	ibpcie_registry_name = string(basic.ConditionTypeIBPcieDowngraded)
)

type ibpcie struct {
	bridge *sop.ApiBridge
}

var ibpcieInstance *ibpcie = &ibpcie{}

func init() {
	nodesop.RegisterSOP(ibpcie_registry_name, ibpcieInstance)
}

func (g *ibpcie) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	ibpcieInstance.bridge = bridge
	return nil
}

func (g *ibpcie) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *ibpcie) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s", node, status.Condition)
	err := basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeIB)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)

	return nil
}
