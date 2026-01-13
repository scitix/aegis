package metrics

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"k8s.io/klog/v2"

	alertv1alpha1 "github.com/scitix/aegis/pkg/apis/alert/v1alpha1"
	"github.com/scitix/aegis/tools"
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

	alertAPIParseSuccessCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "aegis_alert",
		Subsystem: "api",
		Name:      "parse_success_total",
		Help:      "Count of alert messages successfully parsed by API",
	}, []string{"source"})

	alertAPIParseFailureCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "aegis_alert",
		Subsystem: "api",
		Name:      "parse_failure_total",
		Help:      "Count of alert messages failed to parse in API",
	}, []string{"source", "reason"})

	alertAPICreateFailureCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "aegis_alert",
		Subsystem: "api",
		Name:      "create_failure_total",
		Help:      "Count of alert creation failures (CreateOrUpdateAlert returns error)",
	}, []string{"source"})

	alertAPICreateSuccessCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "aegis_alert",
		Subsystem: "api",
		Name:      "create_success_total",
		Help:      "Count of alert creations successfully handled by API",
	}, []string{"source"})
)

func getSubType(alert *alertv1alpha1.AegisAlert) string {
	switch alert.Spec.Type {
	case tools.NodeCriticalIssueType:
		return alert.Spec.Details["condition"]
	default:
		return alert.Spec.Type
	}
}

func (m *MetricsController) RecordAPIParseSuccess(source string) {
	alertAPIParseSuccessCount.WithLabelValues(source).Inc()
}

func (m *MetricsController) RecordAPIParseFailure(source, reason string) {
	alertAPIParseFailureCount.WithLabelValues(source, reason).Inc()
}

func (m *MetricsController) RecordCreateFailure(source string) {
	alertAPICreateFailureCount.WithLabelValues(source).Inc()
}

func (m *MetricsController) RecordCreateSuccess(source string) {
	alertAPICreateSuccessCount.WithLabelValues(source).Inc()
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
	subType := getSubType(alert)

	// Delete alert info metrics
	ret := alertInfo.Delete(prometheus.Labels{
		"name":             alert.Name,
		"namespace":        alert.Namespace,
		"type":             alert.Spec.Type,
		"sub_type":         subType,
		"source":           alert.Spec.Source,
		"status":           string(alert.Spec.Status),
		"object_kind":      string(alert.Spec.InvolvedObject.Kind),
		"object_namespace": alert.Spec.InvolvedObject.Namespace,
		"object_name":      alert.Spec.InvolvedObject.Name,
	})

	if !ret {
		klog.Warningf("failed to delete alert info metrics: %s/%s", alert.Namespace, alert.Name)
	}

	// Delete ops-related metrics using partial match
	alertOpsNotTriggerReason.DeletePartialMatch(prometheus.Labels{
		"name":      alert.Name,
		"namespace": alert.Namespace,
		"type":      alert.Spec.Type,
	})

	alertOpsStatusRunning.DeletePartialMatch(prometheus.Labels{
		"name":      alert.Name,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
	})

	alertOpsStatusFailed.DeletePartialMatch(prometheus.Labels{
		"name":      alert.Name,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
	})

	alertOpsStatuSucceed.DeletePartialMatch(prometheus.Labels{
		"name":      alert.Name,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
	})

	alertOpsExecuteSeconds.DeletePartialMatch(prometheus.Labels{
		"name":      alert.Name,
		"type":      alert.Spec.Type,
		"sub_type":  subType,
		"namespace": alert.Namespace,
	})

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
