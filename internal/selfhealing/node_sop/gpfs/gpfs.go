package gpfs

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

const (
	gpfsdown_registry_name      = string(basic.ConditionTypeGpfsDown)
	gpfsmountlost_registry_name = string(basic.ConditionTypeGpfsMountLost)
	gpfsinactive_registry_name  = string(basic.ConditionTypeGpfsInactive)
)

type gpfs struct {
	bridge *sop.ApiBridge
}

var gpfsInstance *gpfs = &gpfs{}

func init() {
	nodesop.RegisterSOP(gpfsinactive_registry_name, gpfsInstance)
	nodesop.RegisterSOP(gpfsdown_registry_name, gpfsInstance)
	nodesop.RegisterSOP(gpfsmountlost_registry_name, gpfsInstance)
}

func (g *gpfs) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpfsInstance.bridge = bridge
	return nil
}

func (g *gpfs) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpfs) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s and will try to repair.", node, status.Condition)
	customTitle := fmt.Sprintf("aegis detect node %s %s and will try to repair.", node, status.Condition)
	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpfs, customTitle)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")

	// if g.bridge.NodeStatus.RepairCount != nil && *g.bridge.NodeStatus.RepairCount > 5 {
	// 	return nil
	// }

	workflows, _ := g.bridge.TicketManager.GetWorkflows(ctx)
	repairCount := 0
	for _, w := range workflows {
		if w.Action == ticketmodel.TicketWorkflowActionRepair {
			repairCount++
		}
	}

	if repairCount > 5 {
		g.bridge.TicketManager.AddConclusion(ctx, "too many repair")
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	switch status.Condition {
	case gpfsdown_registry_name, gpfsmountlost_registry_name:
		timeOutCtx, cancel := context.WithTimeout(ctx, time.Minute*time.Duration(60))
		defer cancel()

		g.bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionRepair, ticketmodel.TicketWorkflowStatusRunning, nil)
		success, err := basic.RepairNode(timeOutCtx, g.bridge, node)

		if success {
			g.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRepair, ticketmodel.TicketWorkflowStatusSucceeded, nil)
		} else {
			message := fmt.Sprintf("repair failed: %s", err)
			g.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRepair, ticketmodel.TicketWorkflowStatusFailed, &message)
			return err
		}

		g.bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionSleepWait, ticketmodel.TicketWorkflowStatusRunning, nil)
		time.Sleep(basic.SleepWaitDuration)
		g.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionSleepWait, ticketmodel.TicketWorkflowStatusSucceeded, nil)
	}
	return nil
}
