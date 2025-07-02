package gpu

import (
	"context"
	"fmt"
	"time"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/internal/selfhealing/sop/op"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const gpupageretired_registry_name = string(basic.ConditionTypeGpuTooManyPageRetired)

type gpupageretired struct {
	bridge *sop.ApiBridge
}

var gpupageretiredInstance *gpupageretired = &gpupageretired{}

func init() {
	nodesop.RegisterSOP(gpupageretired_registry_name, gpupageretiredInstance)
}

func (g *gpupageretired) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpupageretiredInstance.bridge = bridge
	return nil
}

func (g *gpupageretired) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpupageretired) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s gpu %s %s: %d, requried RMA", node, status.ID, status.Condition, status.Value)
	reason := fmt.Sprintf("aegis detect node %s gpu %s %s: %d, requried RMA", node, status.ID, status.Condition, status.Value)
	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		klog.Warningf("cordon node %s failed", node)
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, reason)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AddWhySRE(ctx, "requrie replace")
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)

	// diagnose
	timeOutCtx, cancel := context.WithTimeout(ctx, time.Minute*time.Duration(20))
	defer cancel()

	err = op.DiagnoseNode(timeOutCtx, g.bridge, node, status.Condition)
	if err != nil {
		klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
		return nil
	}

	return nil
}
