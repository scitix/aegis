package gpfs

import (
	"context"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const (
	gpfsrdmastatuserror_registry_name      = string(basic.ConditionTypeGpfsRdmaStatusError)
	gpfsquorumconnectiondown_registry_name = string(basic.ConditionTypeGpfsQuorumConnectionDown)
	gpfsexpelledfromcluster_registry_name  = string(basic.ConditionTypeGpfsExpelledFromCluster)
	gpfstimeclockerror_registry_name       = string(basic.ConditionTypeGpfsTimeClockError)
	gpfsoslockup_registry_name             = string(basic.ConditionTypeGpfsOsLockup)
	gpfsbadtcpstate_registry_name          = string(basic.ConditionTypeGpfsBadTcpState)
	gpfsunauthorized_registry_name         = string(basic.ConditionTypeGpfsUnauthorized)
	gpfsbond0lostregistry_name             = string(basic.ConditionTypeGpfsBond0Lost)
)

type gpfslogs struct {
	bridge *sop.ApiBridge
}

var gpfslogsInstance *gpfslogs = &gpfslogs{}

func init() {
	nodesop.RegisterSOP(gpfsrdmastatuserror_registry_name, gpfslogsInstance)
	nodesop.RegisterSOP(gpfsquorumconnectiondown_registry_name, gpfslogsInstance)
	nodesop.RegisterSOP(gpfsexpelledfromcluster_registry_name, gpfslogsInstance)
	nodesop.RegisterSOP(gpfstimeclockerror_registry_name, gpfslogsInstance)
	nodesop.RegisterSOP(gpfsoslockup_registry_name, gpfslogsInstance)
	nodesop.RegisterSOP(gpfsbadtcpstate_registry_name, gpfslogsInstance)
	nodesop.RegisterSOP(gpfsunauthorized_registry_name, gpfslogsInstance)
	nodesop.RegisterSOP(gpfsbond0lostregistry_name, gpfslogsInstance)
}

func (g *gpfslogs) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpfslogsInstance.bridge = bridge
	return nil
}

func (g *gpfslogs) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpfslogs) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node gpfs %s %s: %s", node, status.Condition, status.Msg)
	basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpfs)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)

	return nil
}
