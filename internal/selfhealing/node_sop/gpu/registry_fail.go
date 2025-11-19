package gpu

import (
	"context"
	"fmt"
	"strings"
	"time"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/internal/selfhealing/sop/op"
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

const gpuregisterfailed_registry_name = string(basic.ConditionTypeGpuRegisterFailed)

type gpuregisterfail struct {
	bridge *sop.ApiBridge
}

var gpuregisterfailInstance *gpuregisterfail = &gpuregisterfail{}

func init() {
	nodesop.RegisterSOP(gpuregisterfailed_registry_name, gpuregisterfailInstance)
}

func (g *gpuregisterfail) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	gpuregisterfailInstance.bridge = bridge
	return nil
}

func (g *gpuregisterfail) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

// restart nvidia-plugin pod
func (g *gpuregisterfail) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("cordon node: %s, restart nvidia-device-plugin pod and waiting new pod ready for 20m", node)
	err := basic.CordonNode(ctx, g.bridge, node, status.Condition, "aegis")
	if err != nil {
		return err
	}
	
	// check frequency
	if count, err := g.bridge.TicketManager.GetActionCount(ctx, ticketmodel.TicketWorkflowActionRestartPod); err == nil && count > 10 {
		g.bridge.TicketManager.AddConclusion(ctx, "failed after over 10 times success restart gpu plugin")
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	reason := fmt.Sprintf("aegis detect node %s, try to restart nvidia-device-plugin pod and waiting new pod ready for 20m", status.Condition)


	selector := basic.GPUPluginPodSelector
	kv := strings.Split(selector, "=")
	if len(kv) != 2 {
		return fmt.Errorf("invalid gpu plugin pod selector: %s", selector)
	}

	pluginPodReady := basic.IsPodInNodeWithTargetLabelReady(ctx, g.bridge, node, map[string]string{kv[0]: kv[1]})

	if pluginPodReady {
		g.bridge.TicketManager.CreateComponentTicket(ctx,
			fmt.Sprintf("node %s GpuRegisterFailed, lack %d card.", node, status.Value),
			fmt.Sprintf("gpu/%s", basic.ComponentTypeNvidiaDevicePlugin),
			basic.ComponentTypeNvidiaDevicePlugin)
	} else {
		customTitle := fmt.Sprintf("node %s GpuRegisterFailed, lack %d card.", node, status.Value)
		g.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeGpu, customTitle)
	}
	g.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	g.bridge.TicketManager.AdoptTicket(ctx)

	needRestartPluginPod := true
	needRebootNode := false
	needDiaptchTicket := false

	if needRestartPluginPod {
		timeOutCtx, cancel := context.WithTimeout(ctx, time.Duration(20)*time.Minute)
		defer cancel()

		g.bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionRestartPod, ticketmodel.TicketWorkflowStatusRunning, nil)
		err := basic.DeletePodInNodeWithTargetLabel(timeOutCtx, g.bridge, node, map[string]string{kv[0]: kv[1]}, true)
		if err == nil {
			err = basic.WaitPodInNodeWithTargetLabelReady(timeOutCtx, g.bridge, node, map[string]string{kv[0]: kv[1]})
		}

		if err != nil {
			message := fmt.Sprintf("restart failed: %s", err)
			g.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRestartPod, ticketmodel.TicketWorkflowStatusFailed, &message)
			needRebootNode = true
		} else {
			g.bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRestartPod, ticketmodel.TicketWorkflowStatusSucceeded, nil)
		}
	}

	if g.bridge.Aggressive && needRebootNode {
		err := op.RestartNode(ctx, g.bridge, node, reason, canceler)
		if err != nil {
			needDiaptchTicket = true
		}
	}

	if needDiaptchTicket {
		g.bridge.TicketManager.AddConclusion(ctx, "failed selfhealing")
		g.bridge.TicketManager.DispatchTicketToSRE(ctx)
	}

	return nil
}

func (g *gpuregisterfail) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}