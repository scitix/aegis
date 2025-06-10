package models

type AlertSourceType string

const (
	DefaultAlertSource      AlertSourceType = "Default"
	AlertManagerAlertSource AlertSourceType = "AlertManager"
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
	// switch kind {
	// case NodeKind:
	// case PodKind:
	// case ApiServerKind:
	// case EtcdKind:
	// case IngressKind:
	// case WorkflowKind:
	// case KubeletKind:
	// case PrometheusKind:
	// default:
	// 	return fmt.Errorf("invalid kind: %s", kind)
	// }

	return nil
}
