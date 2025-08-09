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
	rocedevicebroken_registry_name = string(basic.ConditionTypeRoceDeviceBroken)
)

type rocedevicebroken struct {
	bridge *sop.ApiBridge
}

var rocedevicebrokenInstance *rocedevicebroken = &rocedevicebroken{}

func init() {
	nodesop.RegisterSOP(rocedevicebroken_registry_name, rocedevicebrokenInstance)
}

func (g *rocedevicebroken) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	rocedevicebrokenInstance.bridge = bridge
	return nil
}

func (g *rocedevicebroken) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return status.Value > 1
}

func (g *rocedevicebroken) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
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
