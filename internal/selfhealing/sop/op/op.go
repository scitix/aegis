package op

import (
	"context"
	"fmt"
	"time"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

func DiagnoseNode(ctx context.Context, bridge *sop.ApiBridge, node, tpe string, params ...string) error {
	// diagnose
	timeOutCtx, cancel := context.WithTimeout(ctx, time.Minute*time.Duration(20))
	defer cancel()

	success, diagnoses, err := basic.DiagnoseNode(timeOutCtx, bridge, node, tpe, params...)
	if err != nil || !success {
		return fmt.Errorf("fail to execute diagnose: %s", err.Error())
	}

	if len(diagnoses) > 0 {
		klog.Infof("diagnose result: %+v", diagnoses)
		bridge.TicketManager.AddDiagnosis(ctx, diagnoses)
	}

	return nil
}

// 1. check running non system pod
// 2. repair
// 3. sleep wait
func RestartNode(ctx context.Context, bridge *sop.ApiBridge, node, reason string, canceler basic.WaitCancelFunc) error {
	if !bridge.Aggressive {
		return fmt.Errorf("cannot restart node because of disable Aggressive mode")
	} else if bridge.AggressiveLevel == 1 {
		cause, _ := bridge.TicketManager.GetRootCauseDescription(ctx)
		if time.Now().Sub(cause.Timestamps) > 48 * time.Hour {
			bridge.TicketManager.AddConclusion(ctx, "ticket deal over 48h. dispatch to sre")
			bridge.TicketManager.DispatchTicketToSRE(ctx)
			return nil
		}	
	} else {
		cause, _ := bridge.TicketManager.GetRootCauseDescription(ctx)
		if time.Now().Sub(cause.Timestamps) > 96 * time.Hour {
			bridge.TicketManager.AddConclusion(ctx, "ticket deal over 96h. dispatch to sre")
			bridge.TicketManager.DispatchTicketToSRE(ctx)
			return nil
		}
	}

	workflows, _ := bridge.TicketManager.GetWorkflows(ctx)
	rebootCount := 0
	for _, w := range workflows {
		if w.Action == ticketmodel.TicketWorkflowActionReboot && w.Status == ticketmodel.TicketWorkflowStatusSucceeded  {
			rebootCount++
		}
	}

	if rebootCount > 1 {
		bridge.TicketManager.AddConclusion(ctx, "too many reboot. perhaps a hardware issue")
		bridge.TicketManager.DispatchTicketToSRE(ctx)
		return nil
	}

	bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionReboot, ticketmodel.TicketWorkflowStatusRunning, nil)

	success, err := basic.NodeGracefulRestart(ctx, bridge, node, reason, "aegis", canceler)
	if success {
		bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionReboot, ticketmodel.TicketWorkflowStatusSucceeded, nil)
		klog.Errorf("reboot success")
	} else if err != nil {
		bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionReboot, ticketmodel.TicketWorkflowStatusFailed, nil)
		klog.Errorf("reboot failed: %s", err)
		return err
	} else {
		bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionReboot, ticketmodel.TicketWorkflowStatusCanceled, nil)
		klog.Warningf("reboot canceled")
		return nil
	}

	timeOutCtx, cancel := context.WithTimeout(ctx, time.Minute*time.Duration(60))
	defer cancel()

	bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionRepair, ticketmodel.TicketWorkflowStatusRunning, nil)
	success, err = basic.RepairNode(timeOutCtx, bridge, node)

	if success {
		bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRepair, ticketmodel.TicketWorkflowStatusSucceeded, nil)
		klog.Infof("repair success")
	} else {
		bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionRepair, ticketmodel.TicketWorkflowStatusFailed, nil)
		klog.Errorf("repair failed: %s", err)
		return err
	}

	bridge.TicketManager.AddWorkflow(ctx, ticketmodel.TicketWorkflowActionSleepWait, ticketmodel.TicketWorkflowStatusRunning, nil)
	time.Sleep(basic.SleepWaitDuration)
	bridge.TicketManager.UpdateWorkflow(ctx, ticketmodel.TicketWorkflowActionSleepWait, ticketmodel.TicketWorkflowStatusSucceeded, nil)
	return nil
}

func ShutdownNode(ctx context.Context, bridge *sop.ApiBridge, node, reason string, canceler basic.WaitCancelFunc) error {
	if !bridge.Aggressive {
		return fmt.Errorf("cannot shutdown node because of disable Aggressive mode")
	}  else if bridge.AggressiveLevel == 1 {
		cause, _ := bridge.TicketManager.GetRootCauseDescription(ctx)
		if time.Now().Sub(cause.Timestamps) > 48 * time.Hour {
			bridge.TicketManager.AddConclusion(ctx, "ticket deal over 48h. dispatch to sre")
			bridge.TicketManager.DispatchTicketToSRE(ctx)
			return nil
		}	
	} else {
		cause, _ := bridge.TicketManager.GetRootCauseDescription(ctx)
		if time.Now().Sub(cause.Timestamps) > 96 * time.Hour {
			bridge.TicketManager.AddConclusion(ctx, "ticket deal over 96h. dispatch to sre")
			bridge.TicketManager.DispatchTicketToSRE(ctx)
			return nil
		}
	}

	bridge.TicketManager.AddShutdownDescription(ctx, ticketmodel.TicketWorkflowStatusRunning, nil)

	success, err := basic.NodeGracefulShutdown(ctx, bridge, node, reason, "aegis", canceler)
	if success {
		bridge.TicketManager.UpdateShutdownDescription(ctx, ticketmodel.TicketWorkflowStatusSucceeded, nil)
	} else if err != nil {
		message := fmt.Sprintf("shutdown failed: %s", err)
		bridge.TicketManager.UpdateShutdownDescription(ctx, ticketmodel.TicketWorkflowStatusFailed, &message)
		return err
	} else {
		message := "shutdown canceled"
		bridge.TicketManager.AddShutdownDescription(ctx, ticketmodel.TicketWorkflowStatusCanceled, &message)
		return nil
	}

	return nil
}
