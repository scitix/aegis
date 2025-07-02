package gpu

import (
	"context"
	"fmt"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const (
	gpuhwslowdown_registry_name = string(basic.ConditionTypeGpuGpuHWSlowdown)
)

type hwslowdown struct {
	bridge *sop.ApiBridge
}

var hwslowdownInstance *hwslowdown = &hwslowdown{}

func init() {
	nodesop.RegisterSOP(gpuhwslowdown_registry_name, hwslowdownInstance)
}

func (g *hwslowdown) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	hwslowdownInstance.bridge = bridge
	return nil
}

func (g *hwslowdown) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *hwslowdown) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node: %s, go on analysis issues", node)

	reason := fmt.Sprintf("aegis detect node %s %s, gpu: %s hw slow down", node, status.Condition, status.ID)
	basic.CordonNode(ctx, g.bridge, node, reason, "aegis")

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, reason)
	g.bridge.TicketManager.AdoptTicket(ctx)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AddWhySRE(ctx, reason)
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)

	return nil
}
