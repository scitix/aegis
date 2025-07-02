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

const gpurowremapping_registry_name = string(basic.ConditionTypeGpuRowRemappingFailure)

type gpurowremapping struct {
	bridge *sop.ApiBridge
}

var gpurowremappingInstance *gpurowremapping = &gpurowremapping{}

func init() {
	nodesop.RegisterSOP(gpurowremapping_registry_name, gpurowremappingInstance)
}

func (g *gpurowremapping) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpurowremappingInstance.bridge = bridge
	return nil
}

func (g *gpurowremapping) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpurowremapping) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node %s for gpu %s %s: %d", node, status.ID, status.Condition, status.Value)

	reason := fmt.Sprintf("aegis detect node %s gpu %s %s: %d, requried replace", node, status.ID, status.Condition, status.Value)
	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		klog.Warningf("cordon node %s failed, still go on restart dcgm pod", node)
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, reason)
	g.bridge.TicketManager.AdoptTicket(ctx)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AddWhySRE(ctx, "requrie replace")

	// diagnose
	err = op.DiagnoseNode(ctx, g.bridge, node, status.Condition)
	if err != nil {
		klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
	}

	g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	return nil
}
