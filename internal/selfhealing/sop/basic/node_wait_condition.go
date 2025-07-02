package basic

import (
	"context"
	"fmt"
	"time"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	"k8s.io/klog/v2"
)

func WaitConditionForNode(ctx context.Context, bridge *sop.ApiBridge, node, condition string) bool {
	timeOutCtx, cancel := context.WithTimeout(ctx, time.Duration(5*24)*time.Hour)
	defer cancel()

	existFunc := func(ctx context.Context) bool {
		query := fmt.Sprintf("aegis_node_status_condition{node=\"%s\", condition=\"%s\"}", node, condition)
		statuses, err := bridge.PromClient.ListNodesWithQuery(ctx, query)
		if err != nil {
			klog.Errorf("error query(%s) from prometheus: %s", err)
			return false
		}

		return len(statuses) > 0
	}

	ticker := time.NewTicker(time.Duration(10) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeOutCtx.Done():
			return false
		case <-ticker.C:
			if existFunc(timeOutCtx) {
				return true
			}
		}
	}
}
