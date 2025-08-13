package uticket

import "os"

const (
	DefaultTicketSupervisorSRE = "admin"
	TicketSupervisorAegis      = "aegis"
)

// GetTicketSupervisorSRE gets default SRE supervisor.
func GetTicketSupervisorSRE() string {
	if s := os.Getenv("SRE"); s != "" {
		return s
	}
	return DefaultTicketSupervisorSRE
}