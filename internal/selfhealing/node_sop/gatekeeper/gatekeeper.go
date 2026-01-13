package gatekeeper

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// getNodesDisableRatio returns the nodes disable ratio from environment variable,
// defaulting to 0.2 if not set or invalid
func getNodesDisableRatio() float64 {
	if ratioStr := os.Getenv("AEGIS_GATEKEEPER_DISABLE_RATIO"); ratioStr != "" {
		if ratio, err := strconv.ParseFloat(ratioStr, 64); err == nil && ratio >= 0 && ratio <= 1 {
			return ratio
		}
		klog.Warningf("Invalid AEGIS_GATEKEEPER_DISABLE_RATIO value: %s, using default 0.1", ratioStr)
	}
	return 0.1
}

type GateKeeper struct {
	bridge            *sop.ApiBridge
	NodesDisableLimit int
}

func CreateGateKeeper(ctx context.Context, bridge *sop.ApiBridge) (*GateKeeper, error) {
	// Initialize GateKeeper without calculating limit upfront
	// Limit will be calculated lazily in Pass() method to save API calls
	return &GateKeeper{
		bridge:            bridge,
		NodesDisableLimit: -1, // -1 indicates limit not calculated yet
	}, nil
}

func (g *GateKeeper) Pass(ctx context.Context) (bool, string) {
	// Add random delay (0-5 seconds) to avoid thundering herd
	randomDelay := time.Duration(rand.Intn(5000)) * time.Millisecond
	klog.V(4).Infof("GateKeeper: applying random delay of %v", randomDelay)
	time.Sleep(randomDelay)

	// Use Kubernetes API to list nodes with cache (single API call)
	allNodes, err := g.bridge.KubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		ResourceVersion: "0", // Force using apiserver cache
	})
	if err != nil {
		return false, fmt.Sprintf("failed to list nodes from apiserver: %v", err)
	}

	// Count cordoned and disabled nodes in single pass
	cordonedCount := 0
	disabledCount := 0

	for _, node := range allNodes.Items {
		// Check if node is cordoned (unschedulable)
		if node.Spec.Unschedulable {
			cordonedCount++
		}

		// Check if node is disabled by aegis
		if node.Labels["aegis.io/disable"] == "true" {
			disabledCount++
		}
	}

	// Calculate available nodes and disable limit (lazy calculation)
	totalNodes := len(allNodes.Items)
	availableNodes := totalNodes - disabledCount
	nodesDisableRatio := getNodesDisableRatio()
	calculatedLimit := int(nodesDisableRatio * float64(availableNodes))

	// Update the limit if not set yet (first call)
	if g.NodesDisableLimit == -1 {
		g.NodesDisableLimit = calculatedLimit
		klog.V(4).Infof("GateKeeper: calculated disable limit: %d (ratio: %.2f, available nodes: %d)",
			calculatedLimit, nodesDisableRatio, availableNodes)
	}

	// Calculate effective cordoned nodes (excluding pre-disabled nodes)
	effectiveCordoned := cordonedCount - disabledCount

	if effectiveCordoned > g.NodesDisableLimit {
		return false, fmt.Sprintf("cluster has %d effectively cordoned nodes (total cordoned: %d, pre-disabled: %d), over the limit: %d",
			effectiveCordoned, cordonedCount, disabledCount, g.NodesDisableLimit)
	}

	klog.V(4).Infof("GateKeeper passed. Total cordoned: %d, pre-disabled: %d, effective cordoned: %d, gate limit: %d",
		cordonedCount, disabledCount, effectiveCordoned, g.NodesDisableLimit)

	return true, ""
}
