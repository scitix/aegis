package sop

import (
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
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

	Registry   string
	Repository string
	OpsImage   string

	KubeClient kubernetes.Interface
	PromClient *prom.PromAPI

	TicketManager ticketmodel.TicketManagerInterface

	EventRecorder record.EventRecorder
}
