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
	xideccmemoryerr_registry_name    = string(basic.ConditionTypeXIDECCMemoryErr)
	xidhwsystemerr_registry_name     = string(basic.ConditionTypeXIDHWSystemErr)
	xidunclassifiederr_registry_name = string(basic.ConditionTypeXIDUnclassifiedErr)
)

type gpu struct {
	bridge *sop.ApiBridge
}

var gpuInstance *gpu = &gpu{}

func init() {
	nodesop.RegisterSOP(xideccmemoryerr_registry_name, gpuInstance)
	nodesop.RegisterSOP(xidhwsystemerr_registry_name, gpuInstance)
	nodesop.RegisterSOP(xidunclassifiederr_registry_name, gpuInstance)
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
	reason := fmt.Sprintf("aegis detect node %s, go on analysis issues", status.Condition)
	if status.ID != "" {
		reason = fmt.Sprintf("%s id: %s", reason, status.ID)
	}

	if status.Value > 1 {
		reason = fmt.Sprintf("%s value: %d", reason, status.Value)
	}

	g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, reason)
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	switch status.Condition {
	case xideccmemoryerr_registry_name:
		if !g.bridge.Aggressive {
			g.bridge.TicketManager.DispatchTicketToSRE(ctx)
			return nil
		}

		err := basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")
		if err != nil {
			return err
		}

		if handler, ok := gpuHandlers[status.Value]; ok {
			return handler(ctx, g.bridge, node, status.ID, status.Value)
		} else {
			return fmt.Errorf("Not found handler for xid: %d", status.Value)
		}
	case xidhwsystemerr_registry_name:
		err := basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")
		if err != nil {
			return err
		}

		// check frequency
		if is, _ := g.bridge.TicketManager.IsFrequentIssue(ctx, 5, 3); is {
			g.bridge.TicketManager.AddWhySRE(ctx, "over 3 same issue for lastest 5 tickets, perhaps a gpu hardware issue.")
			g.bridge.TicketManager.DispatchTicketToSRE(ctx)

			err = op.DiagnoseNode(ctx, g.bridge, node, status.Condition, status.ID, strconv.Itoa(status.Value))
			if err != nil {
				klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
			}
			return nil
		}

		if handler, ok := gpuHandlers[status.Value]; ok {
			err := handler(ctx, g.bridge, node, status.ID, status.Value)
			err = op.DiagnoseNode(ctx, g.bridge, node, status.Condition, status.ID, strconv.Itoa(status.Value))
			if err != nil {
				klog.Errorf("aegis error run diagnose for node %s %s type: %s %s, err: %s", node, status.Condition, status.Type, status.ID, err)
			}

			g.bridge.TicketManager.DispatchTicketToSRE(ctx)
			return err
		} else {
			return fmt.Errorf("Not found handler for xid: %d", status.Value)
		}
	case xidunclassifiederr_registry_name:
	}

	return nil
}
