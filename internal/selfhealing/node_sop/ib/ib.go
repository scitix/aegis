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
	symbolerror_registry_name = string(basic.ConditionTypeIBSymbolError)
)

type symbolerror struct {
	bridge *sop.ApiBridge
}

var symbolerrorInstance *symbolerror = &symbolerror{}

func init() {
	nodesop.RegisterSOP(symbolerror_registry_name, symbolerrorInstance)
}

func (g *symbolerror) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	symbolerrorInstance.bridge = bridge
	return nil
}

func (g *symbolerror) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *symbolerror) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
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
