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
	"k8s.io/klog/v2"
)

const nodeinhibit_registry_name = string(basic.ConditionTypeNodeInhibitAll)

type nodeinhibit struct {
	bridge *sop.ApiBridge
}

var nodeinhibitInstance *nodeinhibit = &nodeinhibit{}

func init() {
	nodesop.RegisterSOP(nodeinhibit_registry_name, nodeinhibitInstance)
}

func (n *nodeinhibit) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	nodeinhibitInstance.bridge = bridge
	return nil
}

func (n *nodeinhibit) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	statuses, err := n.bridge.PromClient.GetNodeStatuses(ctx, node, "")
	if err != nil {
		klog.Warningf("query node status err: %s", err)
		return false
	}

	// aviod not ready node
	for _, s := range statuses {
		if s.Condition == "NodeNotReady" {
			return false
		}
	}

	return true
}

// step 1: run healthcheck sop for node
// if succeed, uncordon node
// if failed, try create ticket or dispatch ticket to sre
func (n *nodeinhibit) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("run healthcheck sop for node: %s", node)

	timeOutCtx, cancel := context.WithTimeout(ctx, time.Minute*time.Duration(20))
	defer cancel()

	operated := false
	workflows, _ := n.bridge.TicketManager.GetWorkflows(ctx)
	for _, wf := range workflows {
		if wf.Status != ticketmodel.TicketWorkflowStatusCanceled && wf.Action != ticketmodel.TicketWorkflowActionHealthCheck {
			operated = true
			break
		}
	}

	n.bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionHealthCheck, ticketmodel.TicketWorkflowStatusRunning, nil)

	success, _, _, err := basic.HealthCheckNode(timeOutCtx, n.bridge, node)
	if err != nil {
		return fmt.Errorf("fail to execute healthcheck: %s", err.Error())
	}

	if success {
		klog.Infof("Node %s healthcheck status succeed", node)
		n.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionHealthCheck, ticketmodel.TicketWorkflowStatusSucceeded, nil)

		err := basic.UncordonNode(timeOutCtx, n.bridge, node, "aegis healthcheck success")
		if err != nil {
			klog.Errorf("Error uncordon node %s: %s", node, err)
			return err
		} else {
			klog.Infof("Succeed uncordon node %s", node)
		}

		// wait to inhibit other alerts
		time.Sleep(basic.SleepWaitDuration)

		n.bridge.TicketManager.AddConclusion(ctx, fmt.Sprintf("succeed run node %s health check", node))

		if operated {
			n.bridge.TicketManager.ResolveTicket(ctx,
				fmt.Sprintf("succeed run node %s health check, so we decide resolve this ticket", node),
				fmt.Sprintf("succeed run node %s health check, so we decide resolve this ticket", node))
		} else {
			n.bridge.TicketManager.CloseTicket(ctx)
		}
	}
	return nil
}
