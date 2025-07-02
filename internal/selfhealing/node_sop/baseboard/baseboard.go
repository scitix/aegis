package baseboard

import (
	"context"
	"fmt"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/internal/selfhealing/sop/op"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const (
	baseboard_registry_name = string(basic.ConditionTypeBaseBoardCriticalIssue)
)

type baseboard struct {
	bridge *sop.ApiBridge
}

var baseboardInstance *baseboard = &baseboard{}

func init() {
	nodesop.RegisterSOP(baseboard_registry_name, baseboardInstance)
}

func (g *baseboard) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	baseboardInstance.bridge = bridge
	return nil
}

func (g *baseboard) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	statuses, err := g.bridge.PromClient.GetNodeStatuses(ctx, node, status.Type)
	if err != nil {
		return false
	}

	// 替换高优 ID
	for _, s := range statuses {
		if status.ID == "sysHealth" && s.ID != status.ID {
			status.ID = s.ID
			status.Value = s.Value
			return true
		}
	}

	return len(statuses) > 0
}

func (g *baseboard) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s %s type: %s %s", node, status.Condition, status.Type, status.ID)

	customTitle := fmt.Sprintf("aegis detect node %s %s type: %s %s from bmc", node, status.Condition, status.Type, status.ID)
	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeBaseBoard, customTitle)
	g.bridge.TicketManager.AdoptTicket(ctx)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AddWhySRE(ctx, "baseboard broken")

	basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")

	// diagnose
	subtype := ""
	switch status.ID {
	case "fan":
		subtype = "Fan"
	case "voltage":
		subtype = "Voltage"
	case "power":
		subtype = "Power"
	case "disk":
		subtype = "Dirve"
	case "temperature":
		subtype = "Temperature"
	case "sysHealth":
		subtype = "Health"
	case "pcie":
		subtype = "PCIe"
	}

	if subtype != "" {
		err := op.DiagnoseNode(ctx, g.bridge, node, status.Condition, subtype)
		if err != nil {
			klog.Infof("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
		}
	}
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)

	return nil
}
