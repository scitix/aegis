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
	gpufabricmanagerinactive_registry_name = string(basic.ConditionTypeNvidiaFabricManagerNotActive)
)

type gpufabricmanagerinactive struct {
	bridge *sop.ApiBridge
}

var gpufabricmanagerinactiveInstance *gpufabricmanagerinactive = &gpufabricmanagerinactive{}

func init() {
	nodesop.RegisterSOP(gpufabricmanagerinactive_registry_name, gpufabricmanagerinactiveInstance)
}

func (g *gpufabricmanagerinactive) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpufabricmanagerinactiveInstance.bridge = bridge
	return nil
}

func (g *gpufabricmanagerinactive) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpufabricmanagerinactive) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s", node, status.Condition)
	reason := fmt.Sprintf("aegis detect node %s %s, begin to restart nvidia-fabricmanager", node, status.Condition)

	err := basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, reason)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	workflows, _ := g.bridge.TicketManager.GetWorkflows(ctx)
	remedyCount := 0
	for _, w := range workflows {
		if w.Action == ticketmodel.TicketWorkflowActionRemedy && w.Status == ticketmodel.TicketWorkflowStatusSucceeded {
			remedyCount++
		}
	}

	if remedyCount > 1 {
		g.bridge.TicketManager.AddWhySRE(ctx, "too many remedy. perhaps a system issue")

		// diagnose
		err := op.DiagnoseNode(ctx, g.bridge, node, status.Condition)
		if err != nil {
			klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
		}

		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	timeOutCtx, cancel := context.WithTimeout(ctx, time.Minute*time.Duration(20))
	defer cancel()
	g.bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionRemedy, ticketmodel.TicketWorkflowStatusRunning, nil)
	success, err := basic.RemedyNode(timeOutCtx, g.bridge, node, basic.RestartFabricmanagerAction)

	if !success {
		g.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRemedy, ticketmodel.TicketWorkflowStatusFailed, nil)
		klog.Warningf("fail to run remedy ops for node %s: %s.", node, err)
	} else {
		g.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRemedy, ticketmodel.TicketWorkflowStatusSucceeded, nil)
	}

	return nil
}
