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

const gpuaggsram_registry_name = string(basic.ConditionTypeGpuAggSramUncorrectable)

type gpuaggsram struct {
	bridge *sop.ApiBridge
}

var gpuaggsramInstance *gpuaggsram = &gpuaggsram{}

func init() {
	nodesop.RegisterSOP(gpuaggsram_registry_name, gpuaggsramInstance)
}

func (g *gpuaggsram) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpuaggsramInstance.bridge = bridge
	return nil
}

func (g *gpuaggsram) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpuaggsram) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node %s for gpu %s %s: %d", node, status.ID, status.Condition, status.Value)

	reason := fmt.Sprintf("aegis detect node %s gpu %s %s: %d, requried replace", node, status.ID, status.Condition, status.Value)
	basic.CordonNode(ctx, g.bridge, node, reason, "aegis")

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, reason)
	g.bridge.TicketManager.AdoptTicket(ctx)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AddWhySRE(ctx, "requrie replace")
	
	if g.bridge.AggressiveLevel > 1 {
		// shutdown
		op.ShutdownNode(ctx, g.bridge, node, "shutdown node for gpu agg sram uncorrectable", canceler)
	}

	g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	return nil
}

func (g *gpuaggsram) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}
