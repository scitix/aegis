package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	nodecheckv1apha1 "github.com/scitix/aegis/pkg/apis/nodecheck/v1alpha1"
)

var (
	nodeHealthCheckStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aegis",
		Subsystem: "node_health",
		Name:      "status",
		Help:      "Status of node health check",
	}, []string{"node", "module", "item", "condition", "level"})
)

func (m *MetricsController) OnNodeCheckUpdate(nodecheck *nodecheckv1apha1.AegisNodeHealthCheck) error {
	if nodecheck.Status.Status != nodecheckv1apha1.CheckStatusSucceeded {
		return nil
	}

	node := nodecheck.Spec.Node

	// clean exists
	// nodeHealthCheckStatus.DeletePartialMatch(prometheus.Labels{
	// 	"node":      node,
	// })

	for module, resourceinfos := range nodecheck.Status.Results {
		for _, info := range resourceinfos {
			status := 0
			if info.Status {
				status = 1
			}
			nodeHealthCheckStatus.With(prometheus.Labels{
				"node":      node,
				"module":    module,
				"item":      info.Item,
				"condition": info.Condition,
				"level":     info.Level,
			}).Set(float64(status))
		}
	}

	return nil
}
