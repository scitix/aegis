package cpu

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

const cpuunhealthy_registry_name = string(basic.ConditionTypeCpuUnhealthy)

type cpuunhealthy struct {
	bridge *sop.ApiBridge
}

var cpuunhealthyInstance *cpuunhealthy = &cpuunhealthy{}

func init() {
	nodesop.RegisterSOP(cpuunhealthy_registry_name, cpuunhealthyInstance)
}

func (n *cpuunhealthy) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	cpuunhealthyInstance.bridge = bridge
	return nil
}

func (n *cpuunhealthy) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (c *cpuunhealthy) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s, sensor name: %s unhealthy from bmc", status.Condition, status.ID)

	customTitle := fmt.Sprintf("aegis detect node %s, sensor name: %s unhealthy from bmc", status.Condition, status.ID)
	c.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeCpu, customTitle)
	c.bridge.TicketManager.AdoptTicket(ctx)
	c.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	c.bridge.TicketManager.AddWhySRE(ctx, "cpu unhealthy")

	basic.CordonNode(ctx, c.bridge, node, status.Condition, "aegis")

	err := op.DiagnoseNode(ctx, c.bridge, node, status.Condition)
	if err != nil {
		klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
	}
	c.bridge.TicketManager.DispatchTicketToSRE(ctx)

	return err
}
