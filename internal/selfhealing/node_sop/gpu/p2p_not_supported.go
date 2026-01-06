package gpu

import (
	"context"
	"fmt"
	"time"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/internal/selfhealing/sop/op"
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

const (
	gpup2pnotsupported_registry_name = string(basic.ConditionTypeGpuP2PNotSupported)
)

type gpup2pnotsupported struct {
	bridge *sop.ApiBridge
}

var gpup2pnotsupportedInstance *gpup2pnotsupported = &gpup2pnotsupported{}

func init() {
	nodesop.RegisterSOP(gpup2pnotsupported_registry_name, gpup2pnotsupportedInstance)
}

func (g *gpup2pnotsupported) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpup2pnotsupportedInstance.bridge = bridge
	return nil
}

func (g *gpup2pnotsupported) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpup2pnotsupported) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node: %s, gpu p2p not supported detected, try to restart node", node)

	reason := fmt.Sprintf("aegis detect node %s %s, gpu p2p not supported, try to restart node", node, status.Condition)
	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, reason)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	// check frequency
	workflows, _ := g.bridge.TicketManager.GetWorkflows(ctx)
	rebootCount := 0
	for _, w := range workflows {
		if w.Action == ticketmodel.TicketWorkflowActionReboot && w.Status == ticketmodel.TicketWorkflowStatusSucceeded {
			rebootCount++
		}
	}

	if rebootCount > 1 {
		g.bridge.TicketManager.AddWhySRE(ctx, "too many reboot. perhaps a hardware issue")
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)

		// diagnose
		err := op.DiagnoseNode(ctx, g.bridge, node, status.Condition)
		if err != nil {
			klog.Errorf("aegis error run diagnose for node %s %s type: %s, err: %s", node, status.Condition, status.Type, err)
		}

		return nil
	}

	if !g.bridge.Aggressive {
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	// restart node
	timeOutCtx, cancel := context.WithTimeout(ctx, time.Minute*time.Duration(30))
	defer cancel()

	g.bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionReboot, ticketmodel.TicketWorkflowStatusRunning, nil)
	err = op.RestartNode(timeOutCtx, g.bridge, node, reason, func(ctx context.Context) bool {
		return false
	})

	if err != nil {
		message := fmt.Sprintf("restart node failed: %s", err)
		g.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionReboot, ticketmodel.TicketWorkflowStatusFailed, &message)
		g.bridge.TicketManager.AddConclusion(ctx, "failed to restart node")
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	} else {
		g.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionReboot, ticketmodel.TicketWorkflowStatusSucceeded, nil)
	}

	return nil
}

func (g *gpup2pnotsupported) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}
