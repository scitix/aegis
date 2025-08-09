package gpu

import (
	"context"
	"fmt"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const (
	highgputemperature_registry_name    = string(basic.ConditionTypeHighGpuMemoryTemp)
	highmemorytemperature_registry_name = string(basic.ConditionTypeHighGpuTemp)
)

type temperature struct {
	bridge *sop.ApiBridge
}

var temperatureInstance *temperature = &temperature{}

func init() {
	nodesop.RegisterSOP(highgputemperature_registry_name, temperatureInstance)
	nodesop.RegisterSOP(highmemorytemperature_registry_name, temperatureInstance)
}

func (g *temperature) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	temperatureInstance.bridge = bridge
	return nil
}

func (g *temperature) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *temperature) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node: %s, go on analysis issues", node)

	reason := fmt.Sprintf("aegis detect node %s %s, gpu: %s, temperature: %d", node, status.Condition, status.ID, status.Value)
	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, reason)
	g.bridge.TicketManager.AdoptTicket(ctx)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AddWhySRE(ctx, "requrie IDC diagnosis")
	g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	return nil
}
