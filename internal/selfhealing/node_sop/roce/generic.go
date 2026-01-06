package roce

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

var genericRoceConditions = []basic.ConditionType{
	basic.ConditionTypeRoceHostOffline,
	basic.ConditionTypeRoceHostGatewayNotMatch,
	basic.ConditionTypeRoceHostRouteMiss,
	basic.ConditionTypeRocePodOffline,
	basic.ConditionTypeRocePodGatewayNotMatch,
	// basic.ConditionTypeRocePodRouteMiss,
	basic.ConditionTypeRoceNodeLabelMiss,
	// basic.ConditionTypeRocePodDeviceMiss,
	basic.ConditionTypeRoceVfDeviceMiss,
	basic.ConditionTypeRoceSriovInitError,
	basic.ConditionTypeRoceNodeUnitLabelMiss,
	basic.ConditionTypeRoceNodePfNamesLabelMiss,
	basic.ConditionTypeRoceNodeResourceLabelMiss,
	basic.ConditionTypeRoceNodeNetworkLabelMiss,
}

type genericRoceHandler struct {
	bridge *sop.ApiBridge
}

var genericRoceHandlerInstance *genericRoceHandler = &genericRoceHandler{}

func init() {
	// Register all generic RoCE conditions with the same handler
	for _, condition := range genericRoceConditions {
		nodesop.RegisterSOP(string(condition), genericRoceHandlerInstance)
	}
}

func (g *genericRoceHandler) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	genericRoceHandlerInstance.bridge = bridge
	return nil
}

func (g *genericRoceHandler) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *genericRoceHandler) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s", node, status.Condition)

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeIB)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	reason := fmt.Sprintf("aegis detect node %s %s", node, status.Condition)
	basic.CordonNode(ctx, g.bridge, node, reason, "aegis")

	description, err := g.bridge.TicketManager.GetRootCauseDescription(ctx)
	if err != nil {
		return err
	}
	startAt := description.Timestamps
	if time.Since(startAt) > 1*time.Hour {
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	}

	return nil
}

func (g *genericRoceHandler) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}
