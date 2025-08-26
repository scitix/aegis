package uticket

import (
	"os"

	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
)

const (
	DefaultTicketSupervisorSRE = "admin"
	TicketSupervisorAegis      = "aegis"
)

// GetTicketSupervisorSRE gets default SRE supervisor.
func GetTicketSupervisorSRE(opts ...string) string {
	sre := os.Getenv("SRE")

	if len(opts) > 0 && opts[0] != basic.ModelTypeHardware {
		sre = os.Getenv("NonHardwareSRE")
	}

	if sre != "" {
		return sre
	}
	return DefaultTicketSupervisorSRE
}