package gpfs

import (
	"context"
	"time"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const (
	gpfscheckfailed_registry_name = string(basic.ConditionTypeGpfsTestFailed)
)

type gpfsbroken struct {
	bridge *sop.ApiBridge
}

var gpfsbrokenInstance *gpfsbroken = &gpfsbroken{}

func init() {
	nodesop.RegisterSOP(gpfscheckfailed_registry_name, gpfsbrokenInstance)
}

func (g *gpfsbroken) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpfsbrokenInstance.bridge = bridge
	return nil
}

func (g *gpfsbroken) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpfsbroken) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s, reason: %s", node, status.Condition, status.Msg)

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpfs)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")

	description, err := g.bridge.TicketManager.GetRootCauseDescription(ctx)
	if err != nil {
		return err
	}
	startAt := description.Timestamps
	if time.Since(startAt) > 24*time.Hour {
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	}

	return nil
}
