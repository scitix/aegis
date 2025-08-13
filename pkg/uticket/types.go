package uticket

type MachineStatus string

const (
	MachinStatusPreMaintain MachineStatus = "preMaintain"
	MachinStatusMaintain    MachineStatus = "maintain"
	MachinStatusHealthy     MachineStatus = "healthy"
)

type MachineInfo struct {
	ClusterName string        `json:"clusterName"`
	NodeName    string        `json:"nodeName"`
	Status      MachineStatus `json:"status"`
	TicketId    string        `json:"ticketID"`
}

type TicketStatus string

const (
	TicketStatusCreated   TicketStatus = "created"
	TicketStatusAssigned  TicketStatus = "assigned"
	TicketStatusResolving TicketStatus = "resolving"
	TicketStatusResolved  TicketStatus = "resolved"
	TicketStatusClosed    TicketStatus = "closed"
)

type TicketInfo struct {
	TicketId        string       `json:"ticketID"`
	Status          TicketStatus `json:"status"`
	Cluster         string       `json:"cluster"`
	Node            string       `json:"node"`
	IsHardwareIssue bool         `json:"isHardwareIssue"`
	HardwareType    bool         `json:"hardwareType"`
	Creator         string       `json:"creator"`
	Description     string       `json:"description"`
	Supervisor      string       `json:"supervisor"`
	Title           string       `json:"title"`
}

type RangeType string

const (
	RangeTypeCluster = "cluster"
	RangeTypeNode    = "node"
	RangeTypeTask    = "task"
	RangeTypePod     = "pod"
)
