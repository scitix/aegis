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
	gpfsrdmaerror_regsitry_name      = string(basic.ConditionTypeGpfsRdmaError)
	gpfsnodenothealthy_registry_name = string(basic.ConditionTypeGpfsNodeNotHealthy)
	gpfsnotmounted_registry_name     = string(basic.ConditionTypeGpfsNotMounted)
	gpfsnotstarted_registry_name     = string(basic.ConditionTypeGpfsNotStarted)
	gpfsnotincluster_registry_name   = string(basic.ConditionTypeGpfsNotInCluster)
	gpfsnotinstalled_registry_name   = string(basic.ConditionTypeGpfsNotInstalled)
)

type gpfsstatus struct {
	bridge *sop.ApiBridge
}

var gpfsstatusInstance *gpfsstatus = &gpfsstatus{}

func init() {
	nodesop.RegisterSOP(gpfsrdmaerror_regsitry_name, gpfsstatusInstance)
	nodesop.RegisterSOP(gpfsnodenothealthy_registry_name, gpfsstatusInstance)
	nodesop.RegisterSOP(gpfsnotmounted_registry_name, gpfsstatusInstance)
	nodesop.RegisterSOP(gpfsnotstarted_registry_name, gpfsstatusInstance)
	nodesop.RegisterSOP(gpfsnotincluster_registry_name, gpfsstatusInstance)
	nodesop.RegisterSOP(gpfsnotinstalled_registry_name, gpfsstatusInstance)
}

func (g *gpfsstatus) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpfsstatusInstance.bridge = bridge
	return nil
}

func (g *gpfsstatus) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpfsstatus) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node gpfs %s %s: %s", node, status.Condition, status.Msg)
	basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpfs)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	description, err := g.bridge.TicketManager.GetRootCauseDescription(ctx)
	if err != nil {
		return err
	}

	startAt := description.Timestamps
	if time.Since(startAt) > 24*time.Hour {
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	}

	g.bridge.TicketManager.DispatchTicketToSRE(ctx)

	return nil
}

func (g *gpfsstatus) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}