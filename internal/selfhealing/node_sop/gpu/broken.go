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
	gpucheckfailed_registry_name = string(basic.ConditionTypeGpuCheckFailed)
)

type gpubroken struct {
	bridge *sop.ApiBridge
}

var gpubrokenInstance *gpubroken = &gpubroken{}

func init() {
	nodesop.RegisterSOP(gpucheckfailed_registry_name, gpubrokenInstance)
}

func (g *gpubroken) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpubrokenInstance.bridge = bridge
	return nil
}

func (g *gpubroken) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpubroken) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
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

		if g.bridge.AggressiveLevel > 1 {
			// shutdown
			op.ShutdownNode(ctx, g.bridge, node, "shutdown node for gpu broken", canceler)
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

func (g *gpubroken) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}
