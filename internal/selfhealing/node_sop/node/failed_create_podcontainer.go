package node

import (
	"context"
	"fmt"
	"time"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
)

const failedcreatepodcontainer_registry_name = string(basic.ConditionTypeKubeletFailedCreatePodContainer)

type failedcreatepodcontainer struct {
	bridge *sop.ApiBridge
}

var failedcreatepodcontainerInstance *failedcreatepodcontainer = &failedcreatepodcontainer{}

func init() {
	nodesop.RegisterSOP(failedcreatepodcontainer_registry_name, failedcreatepodcontainerInstance)
}

func (n *failedcreatepodcontainer) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	failedcreatepodcontainerInstance.bridge = bridge
	return nil
}

func (n *failedcreatepodcontainer) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (n *failedcreatepodcontainer) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	reason := fmt.Sprintf("aegis detect node %s, we will restart kubelet", status.Condition)
	err := basic.CordonNode(ctx, n.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	n.bridge.TicketManager.CreateComponentTicket(ctx, reason, "kubelet", "kubelet")
	n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	n.bridge.TicketManager.AdoptTicket(ctx)

	workflows, _ := n.bridge.TicketManager.GetWorkflows(ctx)
	remedyCount := 0
	for _, w := range workflows {
		if w.Action == ticketmodel.TicketWorkflowActionRemedy {
			remedyCount++
		}
	}

	if remedyCount > 1 {
		n.bridge.TicketManager.AddConclusion(ctx, "too many remedy workflows")
		n.bridge.TicketManager.DispatchTicketToSRE(ctx)

		return nil
	}

	n.bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionRemedy, ticketmodel.TicketWorkflowStatusRunning, nil)
	success, err := basic.RemedyNode(ctx, n.bridge, node, basic.RestartKubeletAction)
	if success {
		n.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRemedy, ticketmodel.TicketWorkflowStatusSucceeded, nil)
	} else {
		message := fmt.Sprintf("reboot failed: %s", err)
		n.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRemedy, ticketmodel.TicketWorkflowStatusFailed, &message)
		return err
	}

	n.bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionSleepWait, ticketmodel.TicketWorkflowStatusRunning, nil)
	time.Sleep(basic.SleepWaitDuration)
	n.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionSleepWait, ticketmodel.TicketWorkflowStatusSucceeded, nil)
	return nil
}
