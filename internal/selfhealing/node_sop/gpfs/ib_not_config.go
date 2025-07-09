package gpfs

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
	gpfsibnotconfig_registry_name = string(basic.ConditionTypeGpfsIBNotConfig)
)

type gpfsibnotconfig struct {
	bridge *sop.ApiBridge
}

var gpfsibnotconfigInstance *gpfsibnotconfig = &gpfsibnotconfig{}

func init() {
	nodesop.RegisterSOP(gpfsibnotconfig_registry_name, gpfsibnotconfigInstance)
}

func (g *gpfsibnotconfig) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpfsibnotconfigInstance.bridge = bridge
	return nil
}

func (g *gpfsibnotconfig) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpfsibnotconfig) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s", node, status.Condition)

	reason := fmt.Sprintf("aegis detect node %s %s", node, status.Condition)
	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpfs)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AddWhySRE(ctx, reason)
	g.bridge.TicketManager.AdoptTicket(ctx)
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)

	basic.CordonNode(ctx, g.bridge, node, reason, "aegis")

	// diagnose
	err := op.DiagnoseNode(ctx, g.bridge, node, status.Condition)
	if err != nil {
		klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
	}

	return nil
}
