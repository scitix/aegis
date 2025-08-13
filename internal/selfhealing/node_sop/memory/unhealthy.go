package memory

import (
	"context"
	"fmt"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/internal/selfhealing/sop/op"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const memoryunhealthy_registry_name = string(basic.ConditionTypeMemoryUnhealthy)

type memoryunhealthy struct {
	bridge *sop.ApiBridge
}

var memoryunhealthyInstance *memoryunhealthy = &memoryunhealthy{}

func init() {
	nodesop.RegisterSOP(memoryunhealthy_registry_name, memoryunhealthyInstance)
}

func (n *memoryunhealthy) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	memoryunhealthyInstance.bridge = bridge
	return nil
}

func (n *memoryunhealthy) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (n *memoryunhealthy) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s, sensor name: %s unhealthy from bmc", status.Condition, status.ID)
	reason := fmt.Sprintf("aegis detect node %s, sensor name: %s unhealthy from bmc", status.Condition, status.ID)
	n.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeMemory, reason)
	n.bridge.TicketManager.AdoptTicket(ctx)
	n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	n.bridge.TicketManager.AddWhySRE(ctx, "exists broken ram")

	klog.Infof("cordon node: %s", node)
	basic.CordonNode(ctx, n.bridge, node, status.Condition, "aegis")

	// diagnose
	err := op.DiagnoseNode(ctx, n.bridge, node, status.Condition)
	if err != nil {
		klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
	}

	if !n.bridge.Aggressive {
		n.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}
	
	cancelled := false
	if !basic.CheckNodeIsCritical(ctx, n.bridge, node) {
		// shutdown
		op.ShutdownNode(ctx, n.bridge, node, "shutdown node for machine repair", func(ctx context.Context) bool {
			statuses, err := n.bridge.PromClient.GetNodeStatuses(ctx, node, status.Type)
			if err == nil && len(statuses) == 0 {
				cancelled = true
				return true
			}
			return false
		})
	}

	if !cancelled {
		n.bridge.TicketManager.DispatchTicketToSRE(ctx)
	}

	return nil
}

func (n *memoryunhealthy) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}

