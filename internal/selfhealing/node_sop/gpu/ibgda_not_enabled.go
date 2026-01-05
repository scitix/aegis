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
	gpuibgdanotenabled_registry_name = string(basic.ConditionTypeGPUIbgdaNotEnabled)
)

type gpuibgdanotenabled struct {
	bridge *sop.ApiBridge
}

var gpuibgdanotenabledInstance *gpuibgdanotenabled = &gpuibgdanotenabled{}

func init() {
	nodesop.RegisterSOP(gpuibgdanotenabled_registry_name, gpuibgdanotenabledInstance)
}

func (g *gpuibgdanotenabled) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpuibgdanotenabledInstance.bridge = bridge
	return nil
}

func (g *gpuibgdanotenabled) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpuibgdanotenabled) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node: %s, gpu ibgda not enabled detected, create ticket and dispatch to SRE", node)

	reason := fmt.Sprintf("aegis detect node %s %s, gpu ibgda not enabled, create ticket and dispatch to SRE", node, status.Condition)
	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	customTitle := fmt.Sprintf("node %s GPUIbgdaNotEnabled detected", node)
	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, customTitle)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)
	g.bridge.TicketManager.AddWhySRE(ctx, "gpu ibgda not enabled, requires SRE intervention")
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)

	// diagnose
	err = op.DiagnoseNode(ctx, g.bridge, node, status.Condition)
	if err != nil {
		klog.Errorf("aegis error run diagnose for node %s %s type: %s, err: %s", node, status.Condition, status.Type, err)
	}

	return nil
}

func (g *gpuibgdanotenabled) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}
