package ib

import (
	"context"
	"fmt"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const (
	iblost_registry_name = string(basic.ConditionTypeIBLost)
)

type iblost struct {
	bridge *sop.ApiBridge
}

var iblostInstance *iblost = &iblost{}

func init() {
	nodesop.RegisterSOP(iblost_registry_name, iblostInstance)
}

func (g *iblost) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	iblostInstance.bridge = bridge
	return nil
}

func (g *iblost) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *iblost) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s", node, status.Condition)

	reason := fmt.Sprintf("aegis detect node %s %s", node, status.Condition)
	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeIB)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	g.bridge.TicketManager.AddConclusion(ctx, "ib lost")
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)

	return nil
}

func (g *iblost) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	reason := fmt.Sprintf("aegis detect node %s %s", node, status.Condition)

	// add ib unavailabel label
	err := basic.AddNodeLabel(ctx, g.bridge, node, basic.NodeIBUnavailableLabelKey, basic.NodeIBUnavailableLabelValue, reason)
	if err != nil {
		return fmt.Errorf("Error add node label %s: %s", node, err)
	}
	return nil
}
