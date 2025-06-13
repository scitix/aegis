package models

type AlertSourceType string

const (
	DefaultAlertSource      AlertSourceType = "Default"
	AlertManagerAlertSource AlertSourceType = "Alertmanager"
	AIAlertSource           AlertSourceType = "AI"
)

const (
	AlertStatusFiring   = "Firing"
	AlertStatusResolved = "Resolved"

	NodeKind       = "Node"
	PodKind        = "Pod"
	ApiServerKind  = "Apiserver"
	EtcdKind       = "Etcd"
	IngressKind    = "Ingress"
	WorkflowKind   = "Workflow"
	KubeletKind    = "Kubelet"
	PrometheusKind = "Prometheus"
)

func validateKind(kind string) error {
	return nil
}
