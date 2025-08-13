package gpu

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

const (
	gpusramuncorrectable_registry_name = string(basic.ConditionTypeGpuVolSramUncorrectable)
)

type volecc struct {
	bridge *sop.ApiBridge
}

var voleccInstance *volecc = &volecc{}

func init() {
	nodesop.RegisterSOP(gpusramuncorrectable_registry_name, voleccInstance)
}

func (g *volecc) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	voleccInstance.bridge = bridge
	return nil
}

func (g *volecc) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *volecc) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node: %s, go on analysis issues", node)

	reason := fmt.Sprintf("aegis detect node %s %s, count: %d", node, status.Condition, status.Value)
	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, reason)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	if !g.bridge.Aggressive {
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	return op.RestartNode(ctx, g.bridge, node, reason, func(ctx context.Context) bool {
		return false
	})
}

func (g *volecc) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}