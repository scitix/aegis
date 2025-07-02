package ib

import (
	"context"
	"fmt"
	"time"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const roceregisterfailed_registry_name = string(basic.ConditionTypeRoceRegisterFailed)

type roceregisterfail struct {
	bridge *sop.ApiBridge
}

var roceregisterfailInstance *roceregisterfail = &roceregisterfail{}

func init() {
	nodesop.RegisterSOP(roceregisterfailed_registry_name, roceregisterfailInstance)
}

func (g *roceregisterfail) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	roceregisterfailInstance.bridge = bridge
	return nil
}

func (g *roceregisterfail) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *roceregisterfail) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node: %s", node)

	reason := fmt.Sprintf("aegis detect node %s, id roce/%s", status.Condition, status.ID)
	basic.CordonNode(ctx, g.bridge, node, reason, "aegis")

	err := g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeIB, reason)
	if err != nil {
		klog.Warningf("create ticket failed: %v", err)
		return err
	}
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AddConclusion(ctx, status.Condition)
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	g.bridge.TicketManager.AdoptTicket(ctx)

	description, err := g.bridge.TicketManager.GetRootCauseDescription(ctx)
	if err != nil {
		return err
	}

	startAt := description.Timestamps
	if time.Since(startAt) > 24*time.Hour {
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	}

	return err
}
