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
	gpurowremappingpending_registry_name = string(basic.ConditionTypeGpuRowRemappingPending)
)

type gpurowremappingpending struct {
	bridge *sop.ApiBridge
}

var gpurowremappingpendingInstance *gpurowremappingpending = &gpurowremappingpending{}

func init() {
	nodesop.RegisterSOP(gpurowremappingpending_registry_name, gpurowremappingpendingInstance)
}

func (g *gpurowremappingpending) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpurowremappingpendingInstance.bridge = bridge
	return nil
}

func (g *gpurowremappingpending) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpurowremappingpending) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s", node, status.Condition)

	reason := fmt.Sprintf("aegis detect node %s %s", node, status.Condition)
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

	// check frequency
	if is, _ := g.bridge.TicketManager.IsFrequentIssue(ctx, 5, 3); is {
		g.bridge.TicketManager.AddWhySRE(ctx, "over 3 same issue for lastest 5 tickets, perhaps a gpu hardware issue.")
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)

		if g.bridge.Aggressive {
			// shutdown
			op.ShutdownNode(ctx, g.bridge, node, "shutdown node for gpu broken", canceler)
		}
		return nil
	}

	return op.RestartNode(ctx, g.bridge, node, reason, func(ctx context.Context) bool {
		return false
	})
}

func (g *gpurowremappingpending) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}