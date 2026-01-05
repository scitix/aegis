package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
)

const terminatingpod_registry_name = string(basic.ConditionTypeNodeHasTerminatingPod)

type terminatingpod struct {
	bridge *sop.ApiBridge
}

var terminatingpodInstance *terminatingpod = &terminatingpod{}

func init() {
	nodesop.RegisterSOP(terminatingpod_registry_name, terminatingpodInstance)
}

func (n *terminatingpod) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	terminatingpodInstance.bridge = bridge
	return nil
}

func (n *terminatingpod) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (n *terminatingpod) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	pods, nil := basic.GetTerminatingPodInNode(ctx, n.bridge, node)
	names := make([]string, 0)
	for _, pod := range pods {
		names = append(names, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
	}

	reason := fmt.Sprintf("aegis detect node %s has stuck in terminating pods(%s) over 20m", node, strings.Join(names, ","))
	err := basic.CordonNode(ctx, n.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	n.bridge.TicketManager.CreateComponentTicket(ctx, reason, basic.ModelTypeKubelet, basic.ComponentTypeKebelet)
	n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	n.bridge.TicketManager.AdoptTicket(ctx)

	description, err := n.bridge.TicketManager.GetRootCauseDescription(ctx)
	if err != nil {
		return err
	}
	startAt := description.Timestamps
	if time.Since(startAt) > 24*time.Hour {
		n.bridge.TicketManager.DispatchTicketToSRE(ctx)
	}

	return nil
}

func (n *terminatingpod) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}
