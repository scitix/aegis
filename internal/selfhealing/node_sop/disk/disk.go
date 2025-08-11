package disk

import (
	"context"
	"fmt"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const diskpressure_registry_name = string(basic.ConditionTypeDiskPressure)

type diskpressure struct {
	bridge *sop.ApiBridge
}

var diskpressureInstance *diskpressure = &diskpressure{}

func init() {
	nodesop.RegisterSOP(diskpressure_registry_name, diskpressureInstance)
}

func (n *diskpressure) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	diskpressureInstance.bridge = bridge
	return nil
}

func (n *diskpressure) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (n *diskpressure) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s disk pressure, used: %d", status.Condition, status.ID, status.Value)

	customTitle := fmt.Sprintf("aegis detect node %s disk pressure, used: %d", status.Condition, status.ID, status.Value)

	n.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeDisk, customTitle)
	n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	n.bridge.TicketManager.AddWhySRE(ctx, "disk pressure")
	n.bridge.TicketManager.DispatchTicketToSRE(ctx)

	basic.CordonNode(ctx, n.bridge, node, status.Condition, "aegis")
	n.bridge.TicketManager.DispatchTicketToSRE(ctx)

	return nil
}