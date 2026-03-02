package nodepoller

import (
	"regexp"

	"github.com/scitix/aegis/pkg/prom"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

var numericNodePattern = regexp.MustCompile(`^[0-9]`)

// classifyResult holds the per-poll classification output.
type classifyResult struct {
	// criticalSet: nodes that have at least one critical condition (priority 0~99)
	// value: the list of AegisNodeStatus for that node
	criticalSet map[string][]prom.AegisNodeStatus

	// cordonOnlySet: nodes that have ONLY NodeCordon (no critical conditions)
	cordonOnlySet map[string]struct{}
}

// classify runs the single PromQL query, filters nodes, and classifies them.
func classify(
	statuses []prom.AegisNodeStatus,
	nodeLister corelisters.NodeLister,
	priority *PriorityWatcher,
) classifyResult {
	klog.V(4).Infof("nodepoller: classify processing %d raw statuses", len(statuses))

	// group statuses by node
	byNode := make(map[string][]prom.AegisNodeStatus)
	for _, s := range statuses {
		byNode[s.Name] = append(byNode[s.Name], s)
	}
	klog.V(4).Infof("nodepoller: grouped into %d unique nodes", len(byNode))

	result := classifyResult{
		criticalSet:   make(map[string][]prom.AegisNodeStatus),
		cordonOnlySet: make(map[string]struct{}),
	}

	skippedNumeric := 0
	skippedNotFound := 0
	skippedDisabled := 0
	processed := 0

	for node, nodeStatuses := range byNode {
		klog.V(6).Infof("nodepoller: processing node %s with %d conditions", node, len(nodeStatuses))

		// filter: skip numeric-prefix nodes
		if numericNodePattern.MatchString(node) {
			klog.V(6).Infof("nodepoller: skipping node %s (numeric prefix)", node)
			skippedNumeric++
			continue
		}

		// filter: skip nodes with aegis.io/disable=true label
		n, err := nodeLister.Get(node)
		if err != nil {
			// node may have disappeared; skip
			klog.V(5).Infof("nodepoller: skipping node %s (not found in lister: %v)", node, err)
			skippedNotFound++
			continue
		}
		if n.Labels["aegis.io/disable"] == "true" {
			klog.V(6).Infof("nodepoller: skipping node %s (aegis.io/disable=true)", node)
			skippedDisabled++
			continue
		}

		hasCritical := false
		hasCordon := false
		criticalConditions := make([]string, 0)
		cordonConditions := make([]string, 0)

		for _, s := range nodeStatuses {
			if priority.IsCritical(s.Condition) {
				hasCritical = true
				criticalConditions = append(criticalConditions, s.Condition)
			}
			if priority.IsCordon(s.Condition) {
				hasCordon = true
				cordonConditions = append(cordonConditions, s.Condition)
			}
		}

		processed++
		if hasCritical {
			klog.V(4).Infof("nodepoller: node %s classified as critical (conditions: %v)", node, criticalConditions)
			result.criticalSet[node] = nodeStatuses
		} else if hasCordon {
			klog.V(4).Infof("nodepoller: node %s classified as cordon-only (conditions: %v)", node, cordonConditions)
			result.cordonOnlySet[node] = struct{}{}
		} else {
			klog.V(6).Infof("nodepoller: node %s has no critical/cordon conditions", node)
		}
	}

	klog.V(4).Infof("nodepoller: classify complete - processed=%d, critical=%d, cordon-only=%d, skipped(numeric=%d,notfound=%d,disabled=%d)",
		processed, len(result.criticalSet), len(result.cordonOnlySet), skippedNumeric, skippedNotFound, skippedDisabled)

	return result
}
