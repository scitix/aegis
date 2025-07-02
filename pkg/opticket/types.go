package opticket

import "github.com/scitix/aegis/pkg/ticketmodel"

type CESInstance struct {
	Region     string `json:"region"`
	OrgName    string `json:"orgName"`
	SN         string `json:"sn"`
	IP         string `json:"ip"`
	Status     string `json:"status"`
	InstanceId string `json:"instanceId"`
}

type TicketStatus string

type OpTicket struct {
	TicketId        string                   `json:"ticketID"`
	Status          ticketmodel.TicketStatus `json:"status"`
	Region          string                   `json:"region"`
	OrgName         string                   `json:"orgName"`
	Node            string                   `json:"node"`
	NodeSN          string                   `json:"nodeSN"`
	IsHardwareIssue bool                     `json:"isHardwareIssue"`
	HardwareType    bool                     `json:"hardwareType"`
	Creator         string                   `json:"creator"`
	Description     string                   `json:"description"`
	Supervisor      string                   `json:"supervisor"`
	Title           string                   `json:"title"`
}
