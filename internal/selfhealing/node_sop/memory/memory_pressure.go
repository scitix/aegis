package memory

import (
	"context"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const memorypressure_registry_name = string(basic.ConditionTypeMemoryPressure)

type memorypressure struct {
	bridge *sop.ApiBridge
}

var memorypressureInstance *memorypressure = &memorypressure{}

func init() {
	nodesop.RegisterSOP(memorypressure_registry_name, memorypressureInstance)
}

func (n *memorypressure) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	memorypressureInstance.bridge = bridge
	return nil
}

// gpu full, give up
func (n *memorypressure) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	statuses, err := n.bridge.PromClient.GetNodeGpuStatuses(ctx, node)
	if err != nil {
		klog.Errorf("error get node %s gpu status: %s", node, err)
		return true
	}

	if len(statuses) == 0 {
		return true
	}

	for _, s := range statuses {
		if s.PodName == "" {
			return true
		}
	}

	return false
}

func (n *memorypressure) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s, value: %d", status.Condition, status.Value)
	if n.bridge.Aggressive {
		basic.CordonNode(ctx, n.bridge, node, status.Condition, "aegis")
	}
	return nil
}
