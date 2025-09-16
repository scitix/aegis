package gpu

import (
	"context"
	"fmt"
	"strconv"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/internal/selfhealing/sop/op"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const (
	xid48_registry_name    = string(basic.ConditionTypeXid48GPUMemoryDBE)
	xid63_registry_name     = string(basic.ConditionTypeXid63ECCRowremapperPending)
	xid64_registry_name     = string(basic.ConditionTypeXid64ECCRowremapperFailure)
	xid95_registry_name     = string(basic.ConditionTypeXid95UncontainedECCError)
	xid74_registry_name     = string(basic.ConditionTypeXid74NVLinkError)
	xid79_registry_name     = string(basic.ConditionTypeXid79GPULost)
)

type gpu struct {
	bridge *sop.ApiBridge
}

var gpuInstance *gpu = &gpu{}

func init() {
	nodesop.RegisterSOP(xid48_registry_name, gpuInstance)
	nodesop.RegisterSOP(xid63_registry_name, gpuInstance)
	nodesop.RegisterSOP(xid64_registry_name, gpuInstance)
	nodesop.RegisterSOP(xid95_registry_name, gpuInstance)
	nodesop.RegisterSOP(xid74_registry_name, gpuInstance)
	nodesop.RegisterSOP(xid79_registry_name, gpuInstance)
}

func (g *gpu) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpuInstance.bridge = bridge
	return nil
}

func (g *gpu) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

func (g *gpu) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s, go on analysis issues", status.Condition)
	reason := fmt.Sprintf("aegis detect node %s, gpu id: %s", status.Condition, status.ID)

	err := basic.CordonNode(ctx, g.bridge, node, reason, "aegis")
	if err != nil {
		return err
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, reason)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	switch status.Condition {
	case xid48_registry_name:
		fallthrough
	case xid63_registry_name:
		fallthrough
	case xid64_registry_name:
		fallthrough
	case xid95_registry_name:
		if !g.bridge.Aggressive {
			g.bridge.TicketManager.DispatchTicketToSRE(ctx)
			return nil
		}

		// check frequency
		if is, _ := g.bridge.TicketManager.IsFrequentIssue(ctx, 5, 3); is {
			g.bridge.TicketManager.AddWhySRE(ctx, "over 3 same issue for lastest 5 tickets, perhaps a gpu hardware issue.")
			g.bridge.TicketManager.DispatchTicketToSRE(ctx)

			if g.bridge.Aggressive {
				// shutdown
				op.ShutdownNode(ctx, g.bridge, node, "shutdown node for gpu broken", canceler)
			}
			return nil
		}

		return op.RestartNode(ctx, g.bridge, node, reason, canceler)
	case xid74_registry_name:
		fallthrough
	case xid79_registry_name:
		// check frequency
		if is, _ := g.bridge.TicketManager.IsFrequentIssue(ctx, 5, 3); is {
			g.bridge.TicketManager.AddWhySRE(ctx, "over 3 same issue for lastest 5 tickets, perhaps a gpu hardware issue.")
			g.bridge.TicketManager.DispatchTicketToSRE(ctx)

			err = op.DiagnoseNode(ctx, g.bridge, node, status.Condition, status.ID, strconv.Itoa(status.Value))
			if err != nil {
				klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
			}

			if g.bridge.Aggressive {
				// shutdown
				op.ShutdownNode(ctx, g.bridge, node, "shutdown node for gpu broken", canceler)
			}
			return nil
		}

		err = op.DiagnoseNode(ctx, g.bridge, node, status.Condition, status.ID, strconv.Itoa(status.Value))
		if err != nil {
			klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
		}

		if !g.bridge.Aggressive {
			g.bridge.TicketManager.DispatchTicketToSRE(ctx)
			return nil
		}

		if g.bridge.Aggressive {
			// shutdown
			op.ShutdownNode(ctx, g.bridge, node, "shutdown node for gpu broken", canceler)
		} else {
			op.RestartNode(ctx, g.bridge, node, reason, canceler)
		}

		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return err
	}

	return nil
}

func (g *gpu) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}