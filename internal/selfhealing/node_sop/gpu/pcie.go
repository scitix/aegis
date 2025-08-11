package gpu

import (
	"context"
	"fmt"
	"strings"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/internal/selfhealing/sop/op"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const (
	gpupciegen_registry_name = string(basic.ConditionTypeGpuPcieGenDowngraded)
	gpupciewidth_registry_name = string(basic.ConditionTypeGpuPcieWidthDowngraded)
)

type gpupcie struct {
	bridge *sop.ApiBridge
}

var gpupcieInstance *gpupcie = &gpupcie{}

func init() {
	nodesop.RegisterSOP(gpupciegen_registry_name, gpupcieInstance)
	nodesop.RegisterSOP(gpupciewidth_registry_name, gpupcieInstance)
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
	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, customTitle)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AddWhySRE(ctx, "requrie replace")
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)

	err := basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")
	if err != nil {
		return err
	}

	// check pcie status
	statuses, err := g.bridge.PromClient.GetNodeStatuses(ctx, node, "gpu")
	if err != nil {
		return err
	}

	ids := make([]string, 0)
	for _, st := range statuses {
		if st.Condition == status.Condition {
			for _, id := range strings.Split(strings.Trim(strings.SplitN(status.Msg, ":", 2)[1], " "), " ") {
				ids = append(ids, strings.SplitN(id, ":", 2)[1])
			}
		}
	}

	if len(ids) > 0 {
		// diagnose
		err := op.DiagnoseNode(ctx, g.bridge, node, status.Condition, ids...)
		if err != nil {
			klog.Infof("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
		}
	}
	return nil
}
