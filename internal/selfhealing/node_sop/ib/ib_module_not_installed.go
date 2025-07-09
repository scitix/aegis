package ib

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
	ibmodule_registry_name = string(basic.ConditionTypeIBModuleNotInstalled)
)

type ibmodule struct {
	bridge *sop.ApiBridge
}

var ibmoduleInstance *ibmodule = &ibmodule{}

func init() {
	nodesop.RegisterSOP(ibmodule_registry_name, ibmoduleInstance)
}

func (g *ibmodule) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	ibmoduleInstance.bridge = bridge
	return nil
}

func (g *ibmodule) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *ibmodule) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s", node, status.Condition)

	reason := fmt.Sprintf("aegis detect node %s %s", node, status.Condition)

	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeIB)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	return nil
}
