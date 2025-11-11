package gpu

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
	gpunvlinkinactive_registry_name = string(basic.ConditionTypeGpuNvlinkInactive)
	gpunvlinkerror_registry_name = string(basic.ConditionTypeGpuNvlinkError)
)

type gpunvlink struct {
	bridge *sop.ApiBridge
}

var gpunvlinkInstance *gpunvlink = &gpunvlink{}

func init() {
	nodesop.RegisterSOP(gpunvlinkinactive_registry_name, gpunvlinkInstance)
	nodesop.RegisterSOP(gpunvlinkerror_registry_name, gpunvlinkInstance)
}

func (g *gpunvlink) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpunvlinkInstance.bridge = bridge
	return nil
}

func (g *gpunvlink) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpunvlink) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s, try to reboot node", node, status.Condition)

	reason := fmt.Sprintf("aegis detect node %s %s, try to reboot node", node, status.Condition)
	err := basic.CordonNode(ctx, g.bridge, node, status.Condition, reason)
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	workflows, _ := g.bridge.TicketManager.GetWorkflows(ctx)
	rebootCount := 0
	for _, w := range workflows {
		if w.Action == ticketmodel.TicketWorkflowActionReboot && w.Status == ticketmodel.TicketWorkflowStatusSucceeded {
			rebootCount++
		}
	}

	if rebootCount > 1 {
		g.bridge.TicketManager.AddWhySRE(ctx, "too many reboot. perhaps a hardware issue")

		// diagnose
		err := op.DiagnoseNode(ctx, g.bridge, node, status.Condition)
		if err != nil {
			klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
		}

		if g.bridge.Aggressive {
			// shutdown
			return op.ShutdownNode(ctx, g.bridge, node, "shutdown node for nvlink down", canceler)
		}

		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	if !g.bridge.Aggressive {
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	// try to restart 
	return op.RestartNode(ctx, g.bridge, node, reason, func(ctx context.Context) bool {
		return false
	})
}

func (g *gpunvlink) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}