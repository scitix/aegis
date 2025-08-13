package gpu

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
	metricshang_registry_name = string(basic.ConditionTypeGpuMetricsHang)
)

type metricshang struct {
	bridge *sop.ApiBridge
}

var metricshangInstance *metricshang = &metricshang{}

func init() {
	nodesop.RegisterSOP(metricshang_registry_name, metricshangInstance)
}

func (g *metricshang) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	metricshangInstance.bridge = bridge
	return nil
}

func (g *metricshang) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *metricshang) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s", node, status.Condition)

	timeOutCtx, cancel := context.WithTimeout(ctx, time.Duration(20)*time.Minute)
	defer cancel()

	return basic.DeletePodInNodeWithTargetLabel(timeOutCtx, g.bridge, node, map[string]string{"app.kubernetes.io/name": "dcgm-exporter"}, false)
}

func (g *metricshang) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}