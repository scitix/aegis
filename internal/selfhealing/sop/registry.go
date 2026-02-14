package sop

import (
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

type NodeStatus struct {
	CordonReason *string
	RebootCount  *int
	RepairCount  *int
}

type ApiBridge struct {
	ClusterName     string
	Region          string
	AlertName       string
	Aggressive      bool
	AggressiveLevel int

	OpsImage string

	Owner *metav1.OwnerReference

	KubeClient kubernetes.Interface
	PromClient *prom.PromAPI

	TicketManager ticketmodel.TicketManagerInterface

	EventRecorder record.EventRecorder
}
