package gpu

import (
	"context"
	"fmt"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/internal/selfhealing/sop/op"
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

const (
	gpupcielink_registry_name = string(basic.ConditionTypeGpuPcieLinkDegraded)
)

type gpupcie struct {
	bridge *sop.ApiBridge
}

var gpupcieInstance *gpupcie = &gpupcie{}

func init() {
	nodesop.RegisterSOP(gpupcielink_registry_name, gpupcieInstance)
}

func (g *gpupcie) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpupcieInstance.bridge = bridge
	return nil
}

func (g *gpupcie) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpupcie) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s, pci_bdf: %s", node, status.Condition, status.PciBdf)
	customTitle := fmt.Sprintf("aegis detect node %s %s, pci_bdf: %s", node, status.Condition, status.PciBdf)

	err := basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")
	if err != nil {
		return err
	}

	if !g.bridge.TicketManager.CheckTicketExists(ctx) {
		g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, customTitle)
		g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
		g.bridge.TicketManager.AdoptTicket(ctx)
		g.bridge.TicketManager.AddWhySRE(ctx, "requrie replace")
		return nil
	}

	if !g.bridge.Aggressive {
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	}

	workflows, _ := g.bridge.TicketManager.GetWorkflows(ctx)
	rebootCount := 0
	for _, w := range workflows {
		if w.Action == ticketmodel.TicketWorkflowActionReboot && w.Status == ticketmodel.TicketWorkflowStatusSucceeded {
			rebootCount++
		}
	}

	if rebootCount > 0 {
		g.bridge.TicketManager.AddWhySRE(ctx, "pcie downgraded still exists after a reboot.")

		// shutdown
		if g.bridge.Aggressive {
			return op.ShutdownNode(ctx, g.bridge, node, "shutdown node for gpu pcied downgraded", func(ctx context.Context) bool {
				return false
			})
		}

		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	} else {
		if err = op.RestartNode(ctx, g.bridge, node, customTitle, func(ctx context.Context) bool {
			return false
		}); err != nil {
			return err
		}
	}
	return nil
}

func (g *gpupcie) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}
