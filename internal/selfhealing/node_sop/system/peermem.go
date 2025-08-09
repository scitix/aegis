package system

import (
	"context"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const (
	peermemnotready_registry_name  = string(basic.ConditionTypePeerMemModuleNotReady)
	peermemnotconfig_registry_name = string(basic.ConditionTypePeerMemModuleNotConfig)
)

type peermem struct {
	bridge *sop.ApiBridge
}

var peermemInstance *peermem = &peermem{}

func init() {
	nodesop.RegisterSOP(peermemnotconfig_registry_name, peermemInstance)
	nodesop.RegisterSOP(peermemnotready_registry_name, peermemInstance)
}

func (n *peermem) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	peermemInstance.bridge = bridge
	return nil
}

func (n *peermem) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (n *peermem) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s, we will repair", status.Condition)

	// err := basic.CordonNode(ctx, n.bridge, node, status.Condition, "aegis")
	// if err != nil {
	// 	return err
	// }

	switch status.Condition {
	case peermemnotready_registry_name:
		fallthrough
	case peermemnotconfig_registry_name:
		success, err := basic.RemedyNode(ctx, n.bridge, node, basic.PeerMemRemedyAction)
		// create a ticket if failed
		if !success && err == nil {
			klog.Errorf("remedy node %s failed", node)
		}
	}

	return nil
}
