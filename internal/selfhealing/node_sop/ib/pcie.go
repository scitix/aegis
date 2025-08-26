package ib

import (
	"context"
	"fmt"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/internal/selfhealing/sop/op"
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

const (
	ibpciespeed_registry_name = string(basic.ConditionTypeIBPCIeSpeedAbnormal)
	ibpciewidth_registry_name = string(basic.ConditionTypeIBPCIeWidthAbnormal)
)

type ibpcie struct {
	bridge *sop.ApiBridge
}

var ibpcieInstance *ibpcie = &ibpcie{}

func init() {
	nodesop.RegisterSOP(ibpciespeed_registry_name, ibpcieInstance)
	nodesop.RegisterSOP(ibpciewidth_registry_name, ibpcieInstance)
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
	reason := fmt.Sprintf("aegis detect node %s %s", node, status.Condition)
	err := basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeIB)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	if !g.bridge.Aggressive {
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	}

	workflows, _ := g.bridge.TicketManager.GetWorkflows(ctx)
	rebootCount := 0
	for _, w := range workflows {
		if w.Action == ticketmodel.TicketWorkflowActionReboot && w.Status == ticketmodel.TicketWorkflowStatusSucceeded {
			rebootCount++
		}
	}

	if rebootCount > 0 {
		g.bridge.TicketManager.AddWhySRE(ctx, "pcie downgraded still exists after a reboot.")

		// shutdown
		if g.bridge.AggressiveLevel > 1 {
			op.ShutdownNode(ctx, g.bridge, node, "shutdown node for ib pcied downgraded", func(ctx context.Context) bool {
				return false
			})
		}

		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	} else {
		if err = op.RestartNode(ctx, g.bridge, node, reason, func(ctx context.Context) bool {
			return false
		}); err != nil {
			return err
		}
	}

	return nil
}

func (g *ibpcie) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	reason := fmt.Sprintf("aegis detect node %s %s", node, status.Condition)

	// add ib unavailabel label
	err := basic.AddNodeLabel(ctx, g.bridge, node, basic.NodeIBUnavailableLabelKey, basic.NodeIBUnavailableLabelValue, reason)
	if err != nil {
		return fmt.Errorf("Error add node label %s: %s", node, err)
	}
	return nil
}
