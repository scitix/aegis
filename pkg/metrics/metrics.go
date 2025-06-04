package metrics

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	alertv1alpha1 "gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/alert/v1alpha1"
	"gitlab.scitix-inner.ai/k8s/aegis/tools"
)

type MetricsController struct {
}

func NewMetricsController() *MetricsController {
	return &MetricsController{}
}

var (
	alertInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aegis_alert",
		Subsystem: "",
		Name:      "info",
		Help:      "Info of Aegis system alert",
	}, []string{"name", "type", "sub_type", "object_kind", "object_namespace", "object_name", "namespace", "source", "status"})

	alertCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "aegis_alert",
		Subsystem: "",
		Name:      "count",
		Help:      "Count of Aegis alert",
	}, []string{"type", "sub_type", "namespace", "source", "status"})

	alertCreated = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aegis_alert",
		Subsystem: "",
		Name:      "created",
		Help:      "Creation timestamp of aegis alert",
	}, []string{"name", "type", "sub_type", "namespace"})

	alertDeleted = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aegis_alert",
		Subsystem: "",
		Name:      "deleted",
		Help:      "Deletion timestamp of aegis alert",
	}, []string{"name", "type", "sub_type", "namespace"})

	alertOpsNotTriggerReason = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aegis_alert",
		Subsystem: "ops",
		Name:      "not_trigger_reason",
		Help:      "Ops trigger status of aegis alert",
	}, []string{"name", "type", "sub_type", "namespace", "reason"})

	alertOpsStatusRunning = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aegis_alert",
		Subsystem: "ops",
		Name:      "status_running",
		Help:      "Ops running of aegis alert",
	}, []string{"name", "type", "sub_type", "namespace"})

	alertOpsStatusFailed = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aegis_alert",
		Subsystem: "ops",
		Name:      "status_failed",
		Help:      "Ops failed of aegis alert",
	}, []string{"name", "type", "sub_type", "namespace"})

	alertOpsStatuSucceed = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aegis_alert",
		Subsystem: "ops",
		Name:      "status_succeed",
		Help:      "Ops succeed of aegis alert",
	}, []string{"name", "type", "sub_type", "namespace"})

	alertOpsExecuteSeconds = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aegis_alert",
		Subsystem: "ops",
		Name:      "execute_seconds",
		Help:      "Ops execute takes seconds",
	}, []string{"name", "type", "sub_type", "namespace", "status"})
)

func getSubType(alert *alertv1alpha1.AegisAlert) string {
	switch alert.Spec.Type {
	case tools.NodeCriticalIssueType:
		return alert.Spec.Details["condition"]
	default:
		return alert.Spec.Type
	}
}

func (m *MetricsController) OnCreate(alert *alertv1alpha1.AegisAlert) error {
	subType := getSubType(alert)
	alertInfo.With(prometheus.Labels{
		"name":             alert.Name,
		"namespace":        alert.Namespace,
		"type":             alert.Spec.Type,
		"sub_type":         subType,
		"source":           alert.Spec.Source,
		"status":           string(alert.Spec.Status),
		"object_kind":      string(alert.Spec.InvolvedObject.Kind),
		"object_namespace": alert.Spec.InvolvedObject.Namespace,
		"object_name":      alert.Spec.InvolvedObject.Name,
	}).Set(float64(1))

	alertCount.With(prometheus.Labels{
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
		"source":    alert.Spec.Source,
		"status":    string(alert.Spec.Status),
	}).Inc()

	alertCreated.With(prometheus.Labels{
		"name":      alert.Name,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
	}).SetToCurrentTime()

	return nil
}

func (m *MetricsController) OnUpdate(alert *alertv1alpha1.AegisAlert) error {
	return nil
}

func (m *MetricsController) OnDelete(alert *alertv1alpha1.AegisAlert) error {
	// ret := alertInfo.Delete(prometheus.Labels{
	// 	"name":             alert.Name,
	// 	"namespace":        alert.Namespace,
	// 	"type":             alert.Spec.Type,
	// 	"source":           alert.Spec.Source,
	// 	"object_kind":      string(alert.Spec.InvolvedObject.Kind),
	// 	"object_namespace": alert.Spec.InvolvedObject.Namespace,
	// 	"object_name":      alert.Spec.InvolvedObject.Name,
	// })

	// if !ret {
	// 	klog.Errorf("fail to delete alert info metrics: %s/%s", alert.Namespace, alert.Name)
	// }

	// ret = alertCreated.Delete(prometheus.Labels{
	// 	"name":      alert.Name,
	// 	"type":      alert.Spec.Type,
	// 	"namespace": alert.Namespace,
	// })

	// if !ret {
	// 	klog.Errorf("fail to delete alert created metrics: %s/%s", alert.Namespace, alert.Name)
	// }
	subType := getSubType(alert)
	alertDeleted.With(prometheus.Labels{
		"name":      alert.Name,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
	}).SetToCurrentTime()

	// alertOpsNotTriggerReason.DeletePartialMatch(prometheus.Labels{
	// 	"name":      alert.Name,
	// 	"namespace": alert.Namespace,
	// 	"type":      alert.Spec.Type,
	// })

	// alertOpsStatusRunning.Delete(prometheus.Labels{
	// 	"name":      alert.Name,
	// 	"namespace": alert.Namespace,
	// 	"type":      alert.Spec.Type,
	// })

	// alertOpsStatusFailed.Delete(prometheus.Labels{
	// 	"name":      alert.Name,
	// 	"namespace": alert.Namespace,
	// 	"type":      alert.Spec.Type,
	// })

	// alertOpsStatuSucceed.Delete(prometheus.Labels{
	// 	"name":      alert.Name,
	// 	"namespace": alert.Namespace,
	// 	"type":      alert.Spec.Type,
	// })

	return nil
}

func (m *MetricsController) OnNoOpsRule(alert *alertv1alpha1.AegisAlert) error {
	subType := getSubType(alert)
	alertOpsNotTriggerReason.With(prometheus.Labels{
		"name":      alert.Name,
		"namespace": alert.Namespace,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"reason":    "NoOpsRule",
	}).Set(float64(1))

	return nil
}

func (m *MetricsController) OnNoOpsTemplate(alert *alertv1alpha1.AegisAlert) error {
	subType := getSubType(alert)
	alertOpsNotTriggerReason.With(prometheus.Labels{
		"name":      alert.Name,
		"namespace": alert.Namespace,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"reason":    "NoOpsTemplate",
	}).Set(float64(1))

	return nil
}

func (m *MetricsController) OnFailedCreateOpsWorkflow(alert *alertv1alpha1.AegisAlert) error {
	subType := getSubType(alert)
	alertOpsNotTriggerReason.With(prometheus.Labels{
		"name":      alert.Name,
		"namespace": alert.Namespace,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"reason":    "FailedCreateOpsWorkflow",
	}).Set(float64(1))

	return nil
}

func (m *MetricsController) OnSucceedCreateOpsWorkflow(alert *alertv1alpha1.AegisAlert) error {
	subType := getSubType(alert)
	alertOpsStatusRunning.With(prometheus.Labels{
		"name":      alert.Name,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
	}).Set(float64(1))

	return nil
}

func opsSpendTime(alert *alertv1alpha1.AegisAlert) (int, error) {
	if alert.Status.OpsStatus.CompletionTime == nil {
		return 0, errors.New("nil completionTime")
	}

	if alert.Status.OpsStatus.StartTime == nil {
		return 0, errors.New("nil startTime")
	}

	return int(alert.Status.OpsStatus.CompletionTime.Time.Sub(alert.Status.OpsStatus.StartTime.Time).Seconds()), nil
}

func (m *MetricsController) OnOpsWorkflowSucceed(alert *alertv1alpha1.AegisAlert) error {
	subType := getSubType(alert)
	alertOpsStatusRunning.With(prometheus.Labels{
		"name":      alert.Name,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
	}).Set(float64(0))

	alertOpsStatuSucceed.With(prometheus.Labels{
		"name":      alert.Name,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
	}).Set(float64(1))

	diff, err := opsSpendTime(alert)
	if err != nil {
		return err
	}

	alertOpsExecuteSeconds.With(prometheus.Labels{
		"name":      alert.Name,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
		"status":    "Succeed",
	}).Set(float64(diff))

	return nil
}

func (m *MetricsController) OnOpsWorkflowFailed(alert *alertv1alpha1.AegisAlert) error {
	subType := getSubType(alert)
	alertOpsStatusRunning.With(prometheus.Labels{
		"name":      alert.Name,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
	}).Set(float64(0))

	alertOpsStatusFailed.With(prometheus.Labels{
		"name":      alert.Name,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
	}).Set(float64(1))

	diff, err := opsSpendTime(alert)
	if err != nil {
		return err
	}

	alertOpsExecuteSeconds.With(prometheus.Labels{
		"name":      alert.Name,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
		"status":    "Failed",
	}).Set(float64(diff))

	return nil
}
