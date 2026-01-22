package node

import (
	"context"
	"fmt"

	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

const notready_registry_name = string(basic.ConditionTypeNodeNotReady)

type notready struct {
	bridge *sop.ApiBridge
}

var notreadyInstance *notready = &notready{}

func init() {
	nodesop.RegisterSOP(notready_registry_name, notreadyInstance)
}

func (n *notready) CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error {
	notreadyInstance.bridge = bridge
	return nil
}

func (n *notready) Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool {
	return true
}

// try to cordon node
func (n *notready) Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	klog.Infof("aegis detect node %s", status.Condition)
	basic.CordonNode(ctx, n.bridge, node, status.Condition, "aegis")

	n.bridge.TicketManager.CreateTicket(ctx, status, basic.HardwareTypeUnknown)
	n.bridge.TicketManager.AddRootCauseDescription(ctx, status.Condition, status)
	n.bridge.TicketManager.AdoptTicket(ctx)

	nodes, err := n.bridge.PromClient.ListNodesWithQuery(ctx, fmt.Sprintf("aegis_node_status_condition{condition=\"NodeNotReady\", node=\"%s\"} offset 1h", node))
	if err != nil {
		return err
	}

	if len(nodes) == 0 {
		return nil
	}

	// force clean terminating pod
	pods, err := basic.GetTerminatingPodInNode(ctx, n.bridge, node)
	if err != nil {
		klog.Warningf("fail to get terminating pods in node: %s, skip", err)
	} else {
		for _, pod := range pods {
			err = basic.DeletePodForcely(ctx, n.bridge, pod.Namespace, pod.Name)
			if err != nil {
				klog.Warningf("fail to force delete pod %s/%s: %s", pod.Namespace, pod.Name, err)
			} else {
				klog.Infof("succeed force delete pod %s/%s", pod.Namespace, pod.Name)
			}
		}
	}

	if n.bridge.TicketManager.CheckTicketExists(ctx) {
		n.bridge.TicketManager.AddConclusion(ctx, "node not ready over 1h")
		n.bridge.TicketManager.DispatchTicketToSRE(ctx)
	}

	return nil
}

func (n *notready) Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error {
	return nil
}