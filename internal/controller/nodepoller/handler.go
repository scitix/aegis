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
	details := buildDetails(node, statuses)
	id, err := p.createAlert(ctx, node, alertTypeNodeCriticalIssue, details)
	if err != nil {
		return "", err
	}
	klog.Infof("nodepoller: created NodeCriticalIssue alert (id=%s) for node %s", id, node)
	return id, nil
}

// onCordonOnlyRisingEdge creates a NodeCriticalIssueDisappeared AegisAlert.
func (p *NodeStatusPoller) onCordonOnlyRisingEdge(ctx context.Context, node string) (string, error) {
	details := map[string]string{"node": node}
	id, err := p.createAlert(ctx, node, alertTypeNodeCriticalIssueDisappeared, details)
	if err != nil {
		return "", err
	}
	klog.Infof("nodepoller: created NodeCriticalIssueDisappeared alert (id=%s) for node %s", id, node)
	return id, nil
}

// createAlert creates a new AegisAlert, or increments the count of an existing
// in-progress alert of the same node+type. Returns the uuid of the alert.
func (p *NodeStatusPoller) createAlert(ctx context.Context, node, alertType string, details map[string]string) (string, error) {
	if active, err := p.findActiveAlert(ctx, node, alertType); err == nil && active != nil {
		if err := p.incurAlertCount(ctx, active); err != nil {
			klog.Errorf("nodepoller: failed to increment count for active alert %s (node=%s type=%s): %v", active.Name, node, alertType, err)
			return "", err
		}
		klog.Infof("nodepoller: active alert %s found for node %s type %s, incremented count to %d", active.Name, node, alertType, active.Status.Count+1)
		return active.Labels["uuid"], nil
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
	if err := p.alertInterface.CreateAlertWithGenerateName(ctx, p.cfg.PublishNamespace, alert, generateName); err != nil {
		return "", err
	}
	return id, nil
}

// alertExists returns true if the AegisAlert identified by uuid still exists.
func (p *NodeStatusPoller) alertExists(ctx context.Context, alertUUID string) bool {
	req, err := labels.NewRequirement("uuid", selection.Equals, []string{alertUUID})
	if err != nil {
		return false
	}
	sel := labels.NewSelector().Add(*req)
	existing, err := p.alertInterface.ListAlertWithLabelSelector(ctx, p.cfg.PublishNamespace, sel)
	if err != nil {
		klog.V(4).Infof("nodepoller: alertExists list error: %v", err)
		return false
	}
	return len(existing) > 0
}

// findActiveAlert returns the first in-progress AegisAlert for the given
// node+alertType, or nil if none exists. An alert is considered in-progress
// when its OpsStatus is neither Succeeded nor Failed.
func (p *NodeStatusPoller) findActiveAlert(ctx context.Context, node, alertType string) (*alertv1alpha1.AegisAlert, error) {
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
		return nil, err
	}

	for _, a := range existing {
		ops := a.Status.OpsStatus.Status
		if ops != alertv1alpha1.OpsStatusSucceeded && ops != alertv1alpha1.OpsStatusFailed {
			return a, nil
		}
	}
	return nil, nil
}

type patchCountValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value int32  `json:"value"`
}

// incurAlertCount increments the status.count of an existing AegisAlert.
func (p *NodeStatusPoller) incurAlertCount(ctx context.Context, alert *alertv1alpha1.AegisAlert) error {
	patches := []patchCountValue{{
		Op:    "replace",
		Path:  "/status/count",
		Value: alert.Status.Count + 1,
	}}
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		return err
	}
	return p.alertInterface.PatchAlert(ctx, p.cfg.PublishNamespace, alert.Name, patchBytes)
}

func buildDetails(node string, statuses []prom.AegisNodeStatus) map[string]string {
	details := map[string]string{"node": node}
	for i, s := range statuses {
		details[fmt.Sprintf("condition_%d", i)] = s.Condition
		if s.ID != "" {
			details[fmt.Sprintf("condition_%d_id", i)] = s.ID
		}
	}
	return details
}
