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
	// group statuses by node
	byNode := make(map[string][]prom.AegisNodeStatus)
	for _, s := range statuses {
		byNode[s.Name] = append(byNode[s.Name], s)
	}

	result := classifyResult{
		criticalSet:   make(map[string][]prom.AegisNodeStatus),
		cordonOnlySet: make(map[string]struct{}),
	}

	for node, nodeStatuses := range byNode {
		// filter: skip numeric-prefix nodes
		if numericNodePattern.MatchString(node) {
			continue
		}

		// filter: skip nodes with aegis.io/disable=true label
		n, err := nodeLister.Get(node)
		if err != nil {
			// node may have disappeared; skip
			klog.V(5).Infof("nodepoller: node %s not found in lister: %v", node, err)
			continue
		}
		if n.Labels["aegis.io/disable"] == "true" {
			continue
		}

		hasCritical := false
		hasCordon := false

		for _, s := range nodeStatuses {
			if priority.IsCritical(s.Condition) {
				hasCritical = true
			}
			if priority.IsCordon(s.Condition) {
				hasCordon = true
			}
		}

		if hasCritical {
			result.criticalSet[node] = nodeStatuses
		} else if hasCordon {
			result.cordonOnlySet[node] = struct{}{}
		}
	}

	return result
}
