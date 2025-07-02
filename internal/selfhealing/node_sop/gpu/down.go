package gpu

import (
	"context"
	"strconv"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/internal/selfhealing/sop/op"
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

const (
	gpudown_registry_name = string(basic.ConditionTypeGpuDown)
)

type gpudown struct {
	bridge *sop.ApiBridge
}

var gpudownInstance *gpudown = &gpudown{}

func init() {
	nodesop.RegisterSOP(gpudown_registry_name, gpudownInstance)
}

func (g *gpudown) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpudownInstance.bridge = bridge
	return nil
}

func (g *gpudown) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpudown) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s", node, status.Condition)

	err := basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	// check frequency
	if is, _ := g.bridge.TicketManager.IsFrequentIssue(ctx, 5, 3); is {
		g.bridge.TicketManager.AddWhySRE(ctx, "over 3 same issue for lastest 5 tickets, perhaps a gpu hardware issue.")
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)

		// diagnose
		err := op.DiagnoseNode(ctx, g.bridge, node, status.Condition, "8", strconv.Itoa(status.Value))
		if err != nil {
			klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
		}
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
		err := op.DiagnoseNode(ctx, g.bridge, node, status.Condition, "8", strconv.Itoa(status.Value))
		if err != nil {
			klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
		}

		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	if !g.bridge.Aggressive {
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	return op.RestartNode(ctx, g.bridge, node, status.Condition, canceler)
}
