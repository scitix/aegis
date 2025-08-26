package ib

import (
	"context"
	"fmt"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/internal/selfhealing/sop/op"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const (
	ibmodulelost_registry_name = string(basic.ConditionTypeIBModuleLost)
)

type ibmodulelost struct {
	bridge *sop.ApiBridge
}

var ibmodulelostInstance *ibmodulelost = &ibmodulelost{}

func init() {
	nodesop.RegisterSOP(ibmodulelost_registry_name, ibmodulelostInstance)
}

func (g *ibmodulelost) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	ibmodulelostInstance.bridge = bridge
	return nil
}

func (g *ibmodulelost) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *ibmodulelost) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s", node, status.Condition)

	reason := fmt.Sprintf("aegis detect node %s %s, module: %s", node, status.Condition, status.ID)
	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeIB)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	g.bridge.TicketManager.AddConclusion(ctx, "ib module lost")

	// diagnose
	err = op.DiagnoseNode(ctx, g.bridge, node, status.Condition, status.ID)
	if err != nil {
		klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
	}

	g.bridge.TicketManager.DispatchTicketToSRE(ctx)

	return nil
}

func (g *ibmodulelost) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	reason := fmt.Sprintf("aegis detect node %s %s", node, status.Condition)

	// add ib unavailabel label
	err := basic.AddNodeLabel(ctx, g.bridge, node, basic.NodeIBUnavailableLabelKey, basic.NodeIBUnavailableLabelValue, reason)
	if err != nil {
		return fmt.Errorf("Error add node label %s: %s", node, err)
	}
	return nil
}
