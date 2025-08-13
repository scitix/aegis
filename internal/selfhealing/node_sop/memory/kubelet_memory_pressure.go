package memory

import (
	"context"
	"time"

	"k8s.io/klog/v2"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
)

const kubeletmemorypressure_registry_name = string(basic.ConditionTypeKubeletMemoryPressure)

type kubeletmemorypressure struct {
	bridge *sop.ApiBridge
}

var kubeletmemorypressureInstance *kubeletmemorypressure = &kubeletmemorypressure{}

func init() {
	nodesop.RegisterSOP(kubeletmemorypressure_registry_name, kubeletmemorypressureInstance)
}

func (n *kubeletmemorypressure) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	kubeletmemorypressureInstance.bridge = bridge
	return nil
}

// gpu full, give up
func (n *kubeletmemorypressure) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (n *kubeletmemorypressure) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node: %s", node)

	basic.CordonNode(ctx, n.bridge, node, status.Condition, "aegis")

	n.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpfs)
	n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	n.bridge.TicketManager.AdoptTicket(ctx)

	timeOutCtx, cancel := context.WithTimeout(ctx, time.Minute*time.Duration(20))
	defer cancel()
	n.bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionRemedy, ticketmodel.TicketWorkflowStatusRunning, nil)
	success, err := basic.RemedyNode(timeOutCtx, n.bridge, node, basic.DropCacheRemedyAction)

	if !success {
		n.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRemedy, ticketmodel.TicketWorkflowStatusFailed, nil)
		klog.Warningf("fail to run remedy ops for node %s: %s.", node, err)
	} else {
		n.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRemedy, ticketmodel.TicketWorkflowStatusSucceeded, nil)
	}

	return nil
}

func (n *kubeletmemorypressure) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}
