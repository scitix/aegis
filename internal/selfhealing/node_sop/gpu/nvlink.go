package gpu

import (
	"context"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const (
	gpunvlink_registry_name = string(basic.ConditionTypeGpuNvlinkInactive)
)

type gpunvlink struct {
	bridge *sop.ApiBridge
}

var gpunvlinkInstance *gpunvlink = &gpunvlink{}

func init() {
	nodesop.RegisterSOP(gpunvlink_registry_name, gpunvlinkInstance)
}

func (g *gpunvlink) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpunvlinkInstance.bridge = bridge
	return nil
}

func (g *gpunvlink) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpunvlink) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s", node, status.Condition)

	err := basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)

	return nil
}
