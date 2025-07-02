package system

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

const highd_registry_name = string(basic.ConditionTypeHighDProcessesCount)

type highd struct {
	bridge *sop.ApiBridge
}

var highdInstance *highd = &highd{}

func init() {
	nodesop.RegisterSOP(highd_registry_name, highdInstance)
}

func (n *highd) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	highdInstance.bridge = bridge
	return nil
}

func (n *highd) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (n *highd) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node: %s and graceful restart node", node)

	if !n.bridge.Aggressive {
		return nil
	}

	klog.Infof("aegis detect node %s, we will graceful restart node", status.Condition)
	err := basic.CordonNode(ctx, n.bridge, node, status.Condition, "aegis")
	if err != nil {
		return err
	}

	reason := fmt.Sprintf("aegis detect node %s, we will graceful restart node", status.Condition)
	n.bridge.TicketManager.CreateComponentTicket(ctx, reason, "os", "os")
	n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	n.bridge.TicketManager.AdoptTicket(ctx)

	return op.RestartNode(ctx, n.bridge, node, status.Condition, func(ctx context.Context) bool {
		return !n.Evaluate(ctx, node, status)
	})
}
