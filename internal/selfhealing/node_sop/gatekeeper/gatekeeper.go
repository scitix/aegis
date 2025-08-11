package gatekeeper

import (
	"context"
	"fmt"

	"github.com/scitix/aegis/internal/selfhealing"
	"github.com/scitix/aegis/internal/selfhealing/sop"
)

var nodesDisableRatio = 0.3

type GateKeeper struct {
	bridge            *sop.ApiBridge
	NodesDisableLimit int
}

func CreateGateKeeper(ctx context.Context, bridge *sop.ApiBridge) (*GateKeeper, error) {
	disables, err := bridge.PromClient.ListNodeStatusesWithQuery(ctx, "kube_node_labels{label_aegis_io_disable=\"true\"}")
	if err != nil {
		return nil, err
	}

	nodes, err := bridge.PromClient.ListNodesWithQuery(ctx, "kube_node_info")
	if err != nil {
		return nil, err
	}

	return &GateKeeper{
		bridge:            bridge,
		NodesDisableLimit: int(nodesDisableRatio * float64(len(nodes) - len(disables))),
	}, nil
}

func (g *GateKeeper) Pass(ctx context.Context) (bool, string) {
	statuses, err := g.bridge.PromClient.ListNodeStatusesWithCondition(ctx, selfhealing.NodeCordonCondition)
	if err != nil {
		return false, fmt.Sprintf("Error get node cordon list from prometheus: %s", err)
	}

	disables, err := g.bridge.PromClient.ListNodeStatusesWithQuery(ctx, "kube_node_labels{label_aegis_io_disable=\"true\"}")
	if err != nil {
		return false, fmt.Sprintf("Error get node disabled list from prometheus: %s", err)
	}

	if cordonNum := len(statuses); cordonNum - len(disables) > g.NodesDisableLimit {
		return false, fmt.Sprintf("cluster cordon %d node, over the limit: %d", cordonNum, g.NodesDisableLimit)
	}

	return true, ""
}
