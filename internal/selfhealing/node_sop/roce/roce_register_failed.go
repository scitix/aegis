package roce

import (
	"context"
	"fmt"
	"strings"
	"time"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
)

const roceregisterfailed_registry_name = string(basic.ConditionTypeRoceRegisterFailed)

type roceregisterfail struct {
	bridge *sop.ApiBridge
}

var roceregisterfailInstance *roceregisterfail = &roceregisterfail{}

func init() {
	nodesop.RegisterSOP(roceregisterfailed_registry_name, roceregisterfailInstance)
}

func (g *roceregisterfail) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	roceregisterfailInstance.bridge = bridge
	return nil
}

func (g *roceregisterfail) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *roceregisterfail) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	err := basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")
	if err != nil {
		return err
	}

	// check frequency
	if count, err := g.bridge.TicketManager.GetActionCount(ctx, ticketmodel.TicketWorkflowActionRestartPod); err == nil && count > 2 {
		g.bridge.TicketManager.AddConclusion(ctx, "failed after over 2 times success restart roce plugin")
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	timeOutCtx, cancel := context.WithTimeout(ctx, time.Duration(20)*time.Minute)
	defer cancel()

	reason := fmt.Sprintf("aegis detect node %s, try to restart rdma-devices pod and waiting new pod ready for 20m", status.Condition)
	g.bridge.TicketManager.CreateComponentTicket(ctx, reason, fmt.Sprintf("roce/%s", basic.ComponentTypeRoceDevicePlugin), basic.ComponentTypeRoceDevicePlugin)
	count, _ := g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	g.bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionRestartPod, ticketmodel.TicketWorkflowStatusRunning, nil)

	selector := basic.RocePluginPodSelector
	kv := strings.Split(selector, "=")
	if len(kv) != 2 {
		return fmt.Errorf("invalid gpu plugin pod selector: %s", selector)
	}
	err = basic.DeletePodInNodeWithTargetLabel(timeOutCtx, g.bridge, node, map[string]string{kv[0]: kv[1]}, true)
	if err == nil {
		err = basic.WaitPodInNodeWithTargetLabelReady(timeOutCtx, g.bridge, node, map[string]string{kv[0]: kv[1]})
	}

	if err != nil {
		message := fmt.Sprintf("fail to restart roce pod: %s", err)
		g.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRestartPod, ticketmodel.TicketWorkflowStatusFailed, &message)

		if count > 1 {
			g.bridge.TicketManager.AddConclusion(ctx, "failed over 2 times")
			g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		}
	} else {
		g.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRestartPod, ticketmodel.TicketWorkflowStatusSucceeded, nil)
	}
	return err
}

func (n *roceregisterfail) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}
