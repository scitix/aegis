package node

import (
	"context"
	"fmt"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

const nodefrequentdown_registry_name = string(basic.ConditionTypeNodeFrequentDown)

type nodefrequentdown struct {
	bridge *sop.ApiBridge
}

var frequentdownInstance *nodefrequentdown = &nodefrequentdown{}

func init() {
	nodesop.RegisterSOP(nodefrequentdown_registry_name, frequentdownInstance)
}

func (n *nodefrequentdown) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	frequentdownInstance.bridge = bridge
	return nil
}

func (n *nodefrequentdown) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	statuses, err := n.bridge.PromClient.GetNodeStatuses(ctx, node, "")
	if err != nil {
		klog.Warningf("query node status err: %s", err)
		return false
	}

	// aviod restarted node
	for _, s := range statuses {
		if s.Condition == "NodeHasRestarted" {
			return false
		}
	}

	for _, s := range statuses {
		if s.Condition == status.Condition {
			return true
		}
	}
	return false
}

func (n *nodefrequentdown) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	if !n.bridge.Aggressive {
		return nil
	}

	klog.Infof("cordon node: %s and graceful restart node", node)
	basic.CordonNode(ctx, n.bridge, node, status.Condition, "aegis")

	klog.Infof("aegis detect node %s, we will graceful restart node", status.Condition)
	reason := fmt.Sprintf("aegis detect node %s, we will graceful restart node", status.Condition)
	n.bridge.TicketManager.CreateComponentTicket(ctx, reason, "kubelet", "kubelet")
	n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	n.bridge.TicketManager.AdoptTicket(ctx)
	n.bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionReboot, ticketmodel.TicketWorkflowStatusRunning, nil)

	success, err := basic.NodeGracefulRestart(ctx, n.bridge, node, status.Condition, "aegis", func(ctx context.Context) bool {
		return !n.Evaluate(ctx, node, status)
	})

	if success {
		n.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionReboot, ticketmodel.TicketWorkflowStatusSucceeded, nil)
	} else {
		message := fmt.Sprintf("reboot failed: %s", err)
		n.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionReboot, ticketmodel.TicketWorkflowStatusFailed, &message)
		return err
	}

	return nil
}
