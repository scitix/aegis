package nodepoller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	alertv1alpha1 "github.com/scitix/aegis/pkg/apis/alert/v1alpha1"
	"github.com/scitix/aegis/pkg/prom"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/klog/v2"
)

const (
	alertSourcePoller = "Poller"

	alertTypeNodeCriticalIssue            = "NodeCriticalIssue"
	alertTypeNodeCriticalIssueDisappeared = "NodeCriticalIssueDisappeared"
)

// onCriticalRisingEdge creates a NodeCriticalIssue AegisAlert for the given node.
// Returns the uuid identifier used to later check alert existence.
func (p *NodeStatusPoller) onCriticalRisingEdge(ctx context.Context, node string, statuses []prom.AegisNodeStatus) (string, error) {
	klog.V(4).Infof("nodepoller: building critical alert for node %s with %d statuses", node, len(statuses))
	details := buildDetails(node, statuses)
	id, err := p.createAlert(ctx, node, alertTypeNodeCriticalIssue, details)
	if err != nil {
		klog.Errorf("nodepoller: failed to create critical alert for node %s: %v", node, err)
		return "", err
	}
	klog.Infof("nodepoller: created NodeCriticalIssue alert (id=%s) for node %s", id, node)
	return id, nil
}

// onCordonOnlyRisingEdge creates a NodeCriticalIssueDisappeared AegisAlert.
func (p *NodeStatusPoller) onCordonOnlyRisingEdge(ctx context.Context, node string) (string, error) {
	klog.V(4).Infof("nodepoller: building cordon-only alert for node %s", node)
	details := map[string]string{"node": node}
	id, err := p.createAlert(ctx, node, alertTypeNodeCriticalIssueDisappeared, details)
	if err != nil {
		klog.Errorf("nodepoller: failed to create cordon-only alert for node %s: %v", node, err)
		return "", err
	}
	klog.Infof("nodepoller: created NodeCriticalIssueDisappeared alert (id=%s) for node %s", id, node)
	return id, nil
}

// createAlert creates a new AegisAlert, or increments the count of an existing
// in-progress alert of the same node+type. Returns the uuid of the alert.
func (p *NodeStatusPoller) createAlert(ctx context.Context, node, alertType string, details map[string]string) (string, error) {
	klog.V(4).Infof("nodepoller: checking for existing active alert (node=%s, type=%s)", node, alertType)
	if active, err := p.findActiveAlert(ctx, node, alertType); err == nil && active != nil {
		klog.V(4).Infof("nodepoller: found active alert %s for node %s type %s (current count: %d)", active.Name, node, alertType, active.Status.Count)
		if err := p.incurAlertCount(ctx, active); err != nil {
			klog.Errorf("nodepoller: failed to increment count for active alert %s (node=%s type=%s): %v", active.Name, node, alertType, err)
			return "", err
		}
		klog.Infof("nodepoller: active alert %s found for node %s type %s, incremented count to %d", active.Name, node, alertType, active.Status.Count+1)
		return active.Labels["uuid"], nil
	} else if err != nil {
		klog.V(4).Infof("nodepoller: error finding active alert for node %s type %s: %v", node, alertType, err)
	} else {
		klog.V(4).Infof("nodepoller: no active alert found for node %s type %s, creating new", node, alertType)
	}

	id := uuid.New().String()
	lbls := map[string]string{
		"alert-source-type": alertSourcePoller,
		"alert-type":        alertType,
		"alert-status":      string(alertv1alpha1.AlertStatusFiring),
		"node":              node,
		"uuid":              id,
	}

	ttlSuccess := p.cfg.DefaultTTLAfterOpsSucceed
	ttlFailed := p.cfg.DefaultTTLAfterOpsFailed
	ttlNoOps := p.cfg.DefaultTTLAfterNoOps

	alert := &alertv1alpha1.AegisAlert{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      lbls,
			Annotations: p.cfg.SystemParas,
		},
		Spec: alertv1alpha1.AegisAlertSpec{
			TTLStrategy: &alertv1alpha1.TTLStrategy{
				SecondsAfterSuccess: &ttlSuccess,
				SecondsAfterFailure: &ttlFailed,
				SecondsAfterNoOps:   &ttlNoOps,
			},
			Selector: &metav1.LabelSelector{MatchLabels: lbls},
			Source:   alertSourcePoller,
			Type:     alertType,
			Status:   alertv1alpha1.AlertStatusFiring,
			InvolvedObject: alertv1alpha1.AegisAlertObject{
				Kind: alertv1alpha1.NodeKind,
				Name: node,
				Node: node,
			},
			Details: details,
		},
		Status: alertv1alpha1.AegisAlertStatus{
			Status: string(alertv1alpha1.AlertStatusFiring),
			Count:  1,
		},
	}

	generateName := fmt.Sprintf("%s-%s-", strings.ToLower(alertSourcePoller), strings.ToLower(alertType))
	klog.V(4).Infof("nodepoller: creating new alert with generateName=%s, uuid=%s", generateName, id)
	if err := p.alertInterface.CreateAlertWithGenerateName(ctx, p.cfg.PublishNamespace, alert, generateName); err != nil {
		klog.Errorf("nodepoller: failed to create alert (generateName=%s, uuid=%s): %v", generateName, id, err)
		return "", err
	}
	klog.V(4).Infof("nodepoller: successfully created alert with uuid=%s", id)
	return id, nil
}

// activeAlertExists returns true if an active AegisAlert for the given node and alertType exists.
// An alert is considered active when its OpsStatus is neither Succeeded nor Failed.
func (p *NodeStatusPoller) activeAlertExists(ctx context.Context, node, alertType string) bool {
	active, err := p.findActiveAlert(ctx, node, alertType)
	if err != nil {
		klog.V(4).Infof("nodepoller: activeAlertExists error for node=%s type=%s: %v", node, alertType, err)
		return false
	}
	exists := active != nil
	klog.V(6).Infof("nodepoller: activeAlertExists check for node=%s type=%s: %v", node, alertType, exists)
	return exists
}

// findActiveAlert returns the first in-progress AegisAlert for the given
// node+alertType, or nil if none exists. An alert is considered in-progress
// when its OpsStatus is neither Succeeded nor Failed.
func (p *NodeStatusPoller) findActiveAlert(ctx context.Context, node, alertType string) (*alertv1alpha1.AegisAlert, error) {
	klog.V(6).Infof("nodepoller: finding active alert (node=%s, type=%s)", node, alertType)
	nodeReq, err := labels.NewRequirement("node", selection.Equals, []string{node})
	if err != nil {
		return nil, err
	}
	typeReq, err := labels.NewRequirement("alert-type", selection.Equals, []string{alertType})
	if err != nil {
		return nil, err
	}
	sel := labels.NewSelector().Add(*nodeReq, *typeReq)

	existing, err := p.alertInterface.ListAlertWithLabelSelector(ctx, p.cfg.PublishNamespace, sel)
	if err != nil {
		klog.V(4).Infof("nodepoller: list alerts failed (node=%s, type=%s): %v", node, alertType, err)
		return nil, err
	}

	klog.V(6).Infof("nodepoller: found %d existing alerts (node=%s, type=%s)", len(existing), node, alertType)
	for _, a := range existing {
		ops := a.Status.OpsStatus.Status
		klog.V(6).Infof("nodepoller: checking alert %s: opsStatus=%s", a.Name, ops)
		if ops != alertv1alpha1.OpsStatusSucceeded && ops != alertv1alpha1.OpsStatusFailed {
			klog.V(4).Infof("nodepoller: found active alert %s (node=%s, type=%s, ops=%s)", a.Name, node, alertType, ops)
			return a, nil
		}
	}
	klog.V(6).Infof("nodepoller: no active alert found (node=%s, type=%s)", node, alertType)
	return nil, nil
}

type patchCountValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value int32  `json:"value"`
}

// incurAlertCount increments the status.count of an existing AegisAlert.
func (p *NodeStatusPoller) incurAlertCount(ctx context.Context, alert *alertv1alpha1.AegisAlert) error {
	newCount := alert.Status.Count + 1
	klog.V(4).Infof("nodepoller: patching alert %s count: %d -> %d", alert.Name, alert.Status.Count, newCount)
	patches := []patchCountValue{{
		Op:    "replace",
		Path:  "/status/count",
		Value: newCount,
	}}
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		klog.Errorf("nodepoller: failed to marshal patch for alert %s: %v", alert.Name, err)
		return err
	}
	if err := p.alertInterface.PatchAlert(ctx, p.cfg.PublishNamespace, alert.Name, patchBytes); err != nil {
		klog.Errorf("nodepoller: failed to patch alert %s: %v", alert.Name, err)
		return err
	}
	klog.V(6).Infof("nodepoller: successfully patched alert %s count to %d", alert.Name, newCount)
	return nil
}

func buildDetails(node string, statuses []prom.AegisNodeStatus) map[string]string {
	details := map[string]string{"node": node}
	klog.V(6).Infof("nodepoller: building details for node %s with %d statuses", node, len(statuses))
	for i, s := range statuses {
		details[fmt.Sprintf("condition_%d", i)] = s.Condition
		if s.ID != "" {
			details[fmt.Sprintf("condition_%d_id", i)] = s.ID
		}
		klog.V(6).Infof("nodepoller: detail condition_%d=%s, id=%s", i, s.Condition, s.ID)
	}
	return details
}
