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
	gpuerrresetrequired_registry_name = string(basic.ConditionTypeGpuErrResetRequired)
)

type gpuerrresetrequired struct {
	bridge *sop.ApiBridge
}

var gpuerrresetrequiredInstance *gpuerrresetrequired = &gpuerrresetrequired{}

func init() {
	nodesop.RegisterSOP(gpuerrresetrequired_registry_name, gpuerrresetrequiredInstance)
}

func (g *gpuerrresetrequired) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpuerrresetrequiredInstance.bridge = bridge
	return nil
}

func (g *gpuerrresetrequired) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpuerrresetrequired) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s", node, status.Condition)

	reason := fmt.Sprintf("aegis detect node %s %s", node, status.Condition)
	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, reason)
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

		if g.bridge.Aggressive {
			// shutdown
			return op.ShutdownNode(ctx, g.bridge, node, "shutdown node for gpu err reset required", nil)
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

func (g *gpuerrresetrequired) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}
