package cpu

import (
	"context"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const cpupressure_registry_name = string(basic.ConditionTypeCPUPressure)

type cpupressure struct {
	bridge *sop.ApiBridge
}

var cpupressureInstance *cpupressure = &cpupressure{}

func init() {
	nodesop.RegisterSOP(cpupressure_registry_name, cpupressureInstance)
}

func (n *cpupressure) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	cpupressureInstance.bridge = bridge
	return nil
}

// if gpu full, give up
func (n *cpupressure) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	statuses, err := n.bridge.PromClient.GetNodeGpuStatuses(ctx, node)
	if err != nil {
		klog.Infof("error get node %s gpu status: %s", node, err)
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

func (n *cpupressure) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s,", status.Condition)
	if n.bridge.Aggressive {
		return basic.CordonNode(ctx, n.bridge, node, status.Condition, "aegis")
	} else {
		klog.V(2).Infof("skip cordon node %s due to non-aggressive mode", node)
		return nil
	}
}

func (g *cpupressure) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}