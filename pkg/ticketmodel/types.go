package ticketmodel

import (
	"time"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type TicketCause struct {
	Timestamps time.Time   `yaml:"timestamps,omitempty"`
	Cause      string      `yaml:"cause,omitempty"`
	Condtion   interface{} `yaml:"condition,omitempty"`
	Count      int64       `yaml:"count,omitempty"`
}

type TicketWorkflow struct {
	Timestamps time.Time            `yaml:"timestamps,omitempty"`
	Action     TicketWorkflowAction `yaml:"action,omitempty"`
	Status     TicketWorkflowStatus `yaml:"status,omitempty"`
	Message    *string              `yaml:"message,omitempty"`
}

type Diagnose struct {
	Hint   string `yaml:"hint,omitempty"`
	Cmd    string `yaml:"cmd,omitempty"`
	Result string `yaml:"result,omitempty"`
}

type TicketDescription struct {
	Cause      TicketCause      `yaml:"cause,omitempty"`
	Workflows  []TicketWorkflow `yaml:"workflows,omitempty"`
	WhySRE     string           `yaml:"whySRE,omitempty"`
	Diagnosis  []Diagnose       `yaml:"diagnosis,omitempty"`
	Conclusion string           `yaml:"conclusion,omitempty"`
	Notified   *bool            `yaml:"notified,omitempty"`
	ShutDown   *TicketWorkflow  `yaml:"shutdown,omitempty"`
}

type TicketStatus string

const (
	TicketStatusCreated   TicketStatus = "created"
	TicketStatusAssigned  TicketStatus = "assigned"
	TicketStatusResolving TicketStatus = "resolving"
	TicketStatusResolved  TicketStatus = "resolved"
	TicketStatusClosed    TicketStatus = "closed"
)

func (t *TicketDescription) Marshal() ([]byte, error) {
	return yaml.Marshal(t)
}

func (t *TicketDescription) Unmarshal(description []byte) error {
	return yaml.Unmarshal(description, t)
}

type TicketManagerArgs struct {
	Client      kubernetes.Interface
	Node        *corev1.Node
	Region      string
	ClusterName string
	OrgName     string
	NodeName    string
	Ip          string
	User        string
}

type TicketWorkflowStatus string

const (
	TicketWorkflowStatusRunning   TicketWorkflowStatus = "Running"
	TicketWorkflowStatusFailed    TicketWorkflowStatus = "Failed"
	TicketWorkflowStatusCanceled  TicketWorkflowStatus = "Canceled"
	TicketWorkflowStatusSucceeded TicketWorkflowStatus = "Succeeded"
)

type TicketWorkflowAction string

const (
	TicketWorkflowActionCordon        TicketWorkflowAction = "Cordon"
	TicketWorkflowActionUncordon      TicketWorkflowAction = "Uncordon"
	TicketWorkflowActionDrain         TicketWorkflowAction = "Drain"
	TicketWorkflowActionRestartPod    TicketWorkflowAction = "RestartPod"
	TicketWorkflowActionReboot        TicketWorkflowAction = "Reboot"
	TicketWorkflowActionShutdown      TicketWorkflowAction = "Shutdown"
	TicketWorkflowActionSleepWait     TicketWorkflowAction = "SleepWait"
	TicketWorkflowActionRepair        TicketWorkflowAction = "Repair"
	TicketWorkflowActionRemedy        TicketWorkflowAction = "Remedy"
	TicketWorkflowActionDiagnose      TicketWorkflowAction = "Diagnose"
	TicketWorkflowActionPerfGPU       TicketWorkflowAction = "PerfGPU"
	TicketWorkflowActionHealthCheck   TicketWorkflowAction = "HealthCheck"
	TicketWorkflowActionWaitCondition TicketWorkflowAction = "WaitCondition"
)