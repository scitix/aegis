package rule

import (
	"testing"

	ruleapi "gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/rule/v1alpha1"
	"gitlab.scitix-inner.ai/k8s/aegis/pkg/controller"
)

func TestMatchRule(t *testing.T) {
	rules := []ruleapi.AegisAlertCondition{
		{
			Status: "Firing",
			Type:   "NodeOutOfDiskSpace",
		},
	}

	condMap := map[*controller.Condition]bool{
		{
			Type:   "NodeOutOfDiskSpace",
			Status: "Firing",
		}: true,
	}

	for condition, expected := range condMap {
		if expected != matchCondition(condition, rules) {
			t.Fatalf("Condition: %v match expected: %v, but got: %v", condition, expected, !expected)
		}
	}
}
