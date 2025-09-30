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
	gpuhung_registry_name = string(basic.ConditionTypeGpuHung)
)

type gpuhung struct {
	bridge *sop.ApiBridge
}

var gpuhungInstance *gpuhung = &gpuhung{}

func init() {
	nodesop.RegisterSOP(gpuhung_registry_name, gpuhungInstance)
}

func (g *gpuhung) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpuhungInstance.bridge = bridge
	return nil
}

func (g *gpuhung) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpuhung) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s", node, status.Condition)

	reason := fmt.Sprintf("aegis detect node %s %s", node, status.Condition)
	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	if !g.bridge.TicketManager.CheckTicketExists(ctx) {
		g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, reason)
		g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
		g.bridge.TicketManager.AdoptTicket(ctx)
		return nil
	}

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
			return op.ShutdownNode(ctx, g.bridge, node, "shutdown node for gpu hung", canceler)
		}

		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	if !g.bridge.Aggressive {
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	return op.RestartNode(ctx, g.bridge, node, reason, func(ctx context.Context) bool {
		statuses, err := g.bridge.PromClient.GetNodeStatuses(ctx, node, status.Type)
		if err == nil && len(statuses) == 0 {
			return true
		}
		return false
	})
}

func (g *gpuhung) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}