package gpu

import (
	"context"
	"fmt"
	"time"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/internal/selfhealing/sop/op"
	"github.com/scitix/aegis/pkg/ticketmodel"
)

type handler func(context.Context, *sop.ApiBridge, string, string, int) error

var gpuHandlers map[int]handler = map[int]handler{
	48: ResetNodeHandle,
	63: ResetNodeHandle,
	64: ResetNodeHandle,
	74: HWSystemErrHandler,
	79: HWSystemErrHandler,
	92: Xid92ErrHandler,
	94: ResetNodeHandle,
	95: ResetNodeHandle,
}

var canceler basic.WaitCancelFunc = func(ctx context.Context) bool {
	return false
}

func HWSystemErrHandler(ctx context.Context, bridge *sop.ApiBridge, node string, id string, code int) error {
	dayCtx, cancel := context.WithTimeout(ctx, time.Hour*time.Duration(24))
	defer cancel()

	bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionDrain, ticketmodel.TicketWorkflowStatusRunning, nil)

	basic.WaitNodeCriticalPodCompeleted(dayCtx, bridge, node, canceler)
	reason := fmt.Sprintf("aegis detect (node %s, gpu %s, xid %d) hw system err", node, id, code)

	err := basic.DrainNode(ctx, bridge, node, reason, "aegis")

	if err != nil {
		message := fmt.Sprintf("drain node error: %s", err)
		bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionDrain, ticketmodel.TicketWorkflowStatusFailed, &message)
	} else {
		bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionDrain, ticketmodel.TicketWorkflowStatusSucceeded, nil)
	}

	return err
}

func ResetNodeHandle(ctx context.Context, bridge *sop.ApiBridge, node string, id string, code int) error {
	if !bridge.Aggressive {
		return nil
	}

	reason := fmt.Sprintf("aegis detect (node %s, gpu: %s, xid: %d) double bit ecc err", node, id, code)
	return op.RestartNode(ctx, bridge, node, reason, canceler)
}

func Xid92ErrHandler(ctx context.Context, bridge *sop.ApiBridge, node string, id string, code int) error {
	return CollectGpuIssueReport(ctx, bridge, node, id, code)
}

func CollectGpuIssueReport(ctx context.Context, bridge *sop.ApiBridge, node string, id string, code int) error {
	return nil
}
