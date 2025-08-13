package disk

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

const diskunhealthy_registry_name = string(basic.ConditionTypeDiskUnhealthy)

type diskunhealthy struct {
	bridge *sop.ApiBridge
}

var diskunhealthyInstance *diskunhealthy = &diskunhealthy{}

func init() {
	nodesop.RegisterSOP(diskunhealthy_registry_name, diskunhealthyInstance)
}

func (n *diskunhealthy) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	diskunhealthyInstance.bridge = bridge
	return nil
}

func (n *diskunhealthy) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (n *diskunhealthy) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s, sensor name: %s unhealthy from bmc", status.Condition, status.ID)

	customTitle := fmt.Sprintf("aegis detect node %s, sensor name: %s unhealthy from bmc", status.Condition, status.ID)

	n.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeDisk, customTitle)
	n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	n.bridge.TicketManager.AddWhySRE(ctx, "disk unhealthy")
	n.bridge.TicketManager.DispatchTicketToSRE(ctx)

	basic.CordonNode(ctx, n.bridge, node, status.Condition, "aegis")

	// diagnose
	err := op.DiagnoseNode(ctx, n.bridge, node, status.Condition)
	if err != nil {
		klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
	}
	n.bridge.TicketManager.DispatchTicketToSRE(ctx)

	return nil
}

func (n *diskunhealthy) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}