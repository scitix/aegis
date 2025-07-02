package gpu

import (
	"context"
	"fmt"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const (
	persistencemode_registry_name = string(basic.ConditionTypeGPUPersistenceModeNotEnabled)
)

type persistencemode struct {
	bridge *sop.ApiBridge
}

var persistencemodeInstance *persistencemode = &persistencemode{}

func init() {
	nodesop.RegisterSOP(persistencemode_registry_name, persistencemodeInstance)
}

func (g *persistencemode) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	persistencemodeInstance.bridge = bridge
	return nil
}

func (g *persistencemode) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *persistencemode) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node: %s, go on analysis issues", node)

	reason := fmt.Sprintf("aegis detect node %s %s, gpu: %s persistence mode not enabled", node, status.Condition, status.ID)
	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	return nil
}
