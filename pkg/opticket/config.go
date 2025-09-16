package opticket

import (
	"context"
	"os"

	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

const (
	DefaultTicketSupervisorSRE = "admin"
	TicketSupervisorAegis      = "aegis"
)

// GetTicketSupervisorSRE gets default SRE supervisor.
// to HardwareSRE: reboot、shutdown succeed
// to NonHardwareSRE: reboot、shutdown non succeed
// to SRE: other
func (t *OpTicketManager) GetTicketSupervisorSRE(ctx context.Context) string {
	NonHardwareSRE := os.Getenv("NonHardwareSRE")
	HardwareSRE := os.Getenv("HardwareSRE")
	SRE := os.Getenv("SRE")
	if SRE == "" {
		if NonHardwareSRE != "" {
			SRE = NonHardwareSRE
		} else if HardwareSRE != "" {
			SRE = HardwareSRE
		} else {
			SRE = DefaultTicketSupervisorSRE
		}
	}

	if NonHardwareSRE == "" {
		NonHardwareSRE = SRE
	}

	if HardwareSRE == "" {
		HardwareSRE = SRE
	}

	var description ticketmodel.TicketDescription
	err := description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		klog.Errorf("error unmarshal description: %s", err)
		return SRE
	}

	shutdown := description.ShutDown
	if shutdown != nil && shutdown.Action == ticketmodel.TicketWorkflowActionShutdown {
		if shutdown.Status == ticketmodel.TicketWorkflowStatusSucceeded {
			return HardwareSRE
		} else {
			return NonHardwareSRE
		}
	}

	workflows := description.Workflows
	if count := len(workflows); count > 0 {
		w := workflows[count-1]
		if w.Action == ticketmodel.TicketWorkflowActionReboot && w.Status == ticketmodel.TicketWorkflowStatusSucceeded {
			return HardwareSRE
		}

		if w.Action == ticketmodel.TicketWorkflowActionReboot && w.Status != ticketmodel.TicketWorkflowStatusSucceeded {
			return NonHardwareSRE
		}
	}

	return SRE
}
