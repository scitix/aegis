package opticket

import "os"

const (
	DefaultTicketSupervisorSRE = "syuan,cfeng,xfchen"
	TicketSupervisorAegis      = "aegis"
)

// GetTicketSupervisorSRE gets default SRE supervisor.
func GetTicketSupervisorSRE() string {
	if s := os.Getenv("SRE"); s != "" {
		return s
	}
	return DefaultTicketSupervisorSRE
}
