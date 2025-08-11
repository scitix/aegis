package ib

import (
	"context"
	"fmt"
	"time"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

const ibregisterfailed_registry_name = string(basic.ConditionTypeIBRegisterFailed)

type ibregisterfail struct {
	bridge *sop.ApiBridge
}

var ibregisterfailInstance *ibregisterfail = &ibregisterfail{}

func init() {
	nodesop.RegisterSOP(ibregisterfailed_registry_name, ibregisterfailInstance)
}

func (g *ibregisterfail) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	ibregisterfailInstance.bridge = bridge
	return nil
}

func (g *ibregisterfail) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

// restart rdma pod
func (g *ibregisterfail) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node: %s, restart rdma-devices pod and waiting new pod ready for 20m", node)

	// check frequency
	if count, err := g.bridge.TicketManager.GetActionCount(ctx, ticketmodel.TicketWorkflowActionRestartPod); err == nil && count > 10 {
		g.bridge.TicketManager.AddConclusion(ctx, "failed after over 10 times success restart rdma plugin")
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	timeOutCtx, cancel := context.WithTimeout(ctx, time.Duration(20)*time.Minute)
	defer cancel()

	reason := fmt.Sprintf("aegis detect node %s, try to restart rdma-devices pod and waiting new pod ready for 20m", status.Condition)
	g.bridge.TicketManager.CreateComponentTicket(ctx, reason, fmt.Sprintf("ib/%s", basic.ComponentTypeRdmaDevicePlugin), basic.ComponentTypeRdmaDevicePlugin)
	count, _ := g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	g.bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionRestartPod, ticketmodel.TicketWorkflowStatusRunning, nil)

	err := basic.DeletePodInNodeWithTargetLabel(timeOutCtx, g.bridge, node, map[string]string{"name": "rdma-devices-ds-all"}, true)
	if err == nil {
		err = basic.WaitPodInNodeWithTargetLabelReady(timeOutCtx, g.bridge, node, map[string]string{"name": "rdma-devices-ds-all"})
	}

	if err != nil {
		message := fmt.Sprintf("fail to restart rdma pod: %s", err)
		g.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRestartPod, ticketmodel.TicketWorkflowStatusFailed, &message)

		if count > 1 {
			g.bridge.TicketManager.AddConclusion(ctx, "failed over 1 times")
			g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		}
	} else {
		g.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRestartPod, ticketmodel.TicketWorkflowStatusSucceeded, nil)
	}
	return err
}
