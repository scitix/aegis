package node

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

const nodecordon_registry_name = string(basic.ConditionTypeNodeCordon)

type nodecordon struct {
	bridge *sop.ApiBridge
}

var cordonInstance *nodecordon = &nodecordon{}

func init() {
	nodesop.RegisterSOP(nodecordon_registry_name, cordonInstance)
}

func (n *nodecordon) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	cordonInstance.bridge = bridge
	return nil
}

func (n *nodecordon) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

// step 1: run healthcheck sop for node
// if succeed, try resolve ticket
// if failed, try create ticket or dispatch ticket to sre
func (n *nodecordon) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("run healthcheck sop for node: %s", node)

	timeOutCtx, cancel := context.WithTimeout(ctx, time.Minute*time.Duration(20))
	defer cancel()

	operated := false
	rebootCount := 0
	workflows, _ := n.bridge.TicketManager.GetWorkflows(ctx)
	for _, wf := range workflows {
		if wf.Status != ticketmodel.TicketWorkflowStatusCanceled && wf.Action != ticketmodel.TicketWorkflowActionHealthCheck {
			operated = true
		}

		if wf.Action == ticketmodel.TicketWorkflowActionReboot {
			rebootCount++
		}
	}

	n.bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionHealthCheck, ticketmodel.TicketWorkflowStatusRunning, nil)

	var result string
	success, hardwareType, conditionType, err := basic.HealthCheckNode(timeOutCtx, n.bridge, node)
	if err != nil {
		return fmt.Errorf("fail to execute healthcheck: %s", err.Error())
	}

	// success
	if success {
		klog.Infof("Node %s healthcheck status succeeded", node)
		n.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionHealthCheck, ticketmodel.TicketWorkflowStatusSucceeded, nil)

		err := basic.UncordonNode(timeOutCtx, n.bridge, node, "aegis healthcheck success")
		if err != nil {
			klog.Errorf("Error uncordon node %s: %s", node, err)
			return err
		} else {
			klog.Infof("Succeed uncordon node %s", node)
		}

		n.bridge.TicketManager.AddConclusion(ctx, fmt.Sprintf("succeed run node %s health check", node))

		if operated {
			n.bridge.TicketManager.ResolveTicket(ctx,
				fmt.Sprintf("succeed run node %s health check, so we decide resolve this ticket", node),
				fmt.Sprintf("succeed run node %s health check, so we decide resolve this ticket", node))
		} else {
			n.bridge.TicketManager.CloseTicket(ctx)
		}

		return nil
	}

	// healthcheck failed
	klog.Errorf("healthcheck sop get err result for node: %s, begin to create a ticket", node)

	// gpu remapping failure
	if hardwareType == basic.HardwareTypeGpu && conditionType == basic.ConditionTypeGpuRowRemappingFailure {
		n.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionHealthCheck, ticketmodel.TicketWorkflowStatusFailed, &result)
		n.bridge.TicketManager.AddWhySRE(ctx, "gpu remapping failure, require replace")

		// diagnose
		err := op.DiagnoseNode(ctx, n.bridge, node, string(conditionType))
		if err != nil {
			klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
		}

		n.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	// gpu too many sram uncorrectable error
	if hardwareType == basic.HardwareTypeGpu && conditionType == basic.ConditionTypeGpuSramUncorrectable {
		if !n.bridge.TicketManager.CheckTicketExists(ctx) {
			customTitle := fmt.Sprintf("node %s healthcheck find gpu too many sram uncorrectable error", node)
			n.bridge.TicketManager.CreateTicket(ctx, status, string(hardwareType), customTitle)
			n.bridge.TicketManager.AddRootCauseDescription(ctx, fmt.Sprintf("%s broken", string(hardwareType)), status)
		}

		n.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionHealthCheck, ticketmodel.TicketWorkflowStatusFailed, &result)
		n.bridge.TicketManager.AddWhySRE(ctx, "gpu too many sram uncorrectable error, require replace")

		// diagnose
		err := op.DiagnoseNode(ctx, n.bridge, node, string(conditionType))
		if err != nil {
			klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
		}

		n.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	if !n.bridge.TicketManager.CheckTicketExists(ctx) {
		customTitle := fmt.Sprintf("node %s healthcheck find %s broken", node, hardwareType)
		n.bridge.TicketManager.CreateTicket(ctx, status, string(hardwareType), customTitle)
		n.bridge.TicketManager.AddRootCauseDescription(ctx, fmt.Sprintf("%s broken", string(hardwareType)), status)
		n.bridge.TicketManager.AdoptTicket(ctx)
	}
	n.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionHealthCheck, ticketmodel.TicketWorkflowStatusFailed, &result)
	n.bridge.TicketManager.AddWhySRE(ctx, fmt.Sprintf("failed run node %s health check", node))
	n.bridge.TicketManager.DispatchTicketToSRE(ctx)

	return nil
}
