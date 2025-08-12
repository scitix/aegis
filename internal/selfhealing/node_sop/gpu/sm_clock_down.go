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
	smclockslowdown_registry_name = string(basic.ConditionTypeGpuSmClkSlowDown)
)

type smclockslowdown struct {
	bridge *sop.ApiBridge
}

var smclockslowdownInstance *smclockslowdown = &smclockslowdown{}

func init() {
	nodesop.RegisterSOP(smclockslowdown_registry_name, smclockslowdownInstance)
}

func (g *smclockslowdown) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	smclockslowdownInstance.bridge = bridge
	return nil
}

func (g *smclockslowdown) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *smclockslowdown) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node: %s, go on analysis issues", node)

	reason := fmt.Sprintf("aegis detect node %s %s, value: %d MHZ", node, status.Condition, status.Value)
	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, reason)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)

	return nil
}
