package alert

import (
	"testing"

	v1alpha1 "gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/alert/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

func TestIsAlertFinished(t *testing.T) {
	testCases := map[string]struct {
		conditionType          v1alpha1.AlertOpsConditionType
		conditionStatus        v1.ConditionStatus
		expectAlertNotFinished bool
	}{
		"Alert OpsWorkflow is completed and condition is true": {
			v1alpha1.AlertCompleteOpsWrofklow,
			v1.ConditionTrue,
			false,
		},
		"Alert OpsWorkflow is completed and condition is false": {
			v1alpha1.AlertCompleteOpsWrofklow,
			v1.ConditionFalse,
			true,
		},
		"Alert OpsWorkflow is completed and condition is unknown": {
			v1alpha1.AlertCompleteOpsWrofklow,
			v1.ConditionUnknown,
			true,
		},
		"Alert OpsWorkflow is failed and condition is true": {
			v1alpha1.AlertFailedOpsWrofklow,
			v1.ConditionTrue,
			false,
		},
		"Alert OpsWorkflow is failed and condition is false": {
			v1alpha1.AlertFailedOpsWrofklow,
			v1.ConditionFalse,
			true,
		},
		"Alert OpsWorkflow is failed and condition is unknown": {
			v1alpha1.AlertFailedOpsWrofklow,
			v1.ConditionUnknown,
			true,
		},
		"Alert CreateOpsWorkflow is failed and condition is true": {
			v1alpha1.AlertFailedCreateOpsWorkflow,
			v1.ConditionTrue,
			false,
		},
	}

	for name, tc := range testCases {
		alert := &v1alpha1.AegisAlert{
			Status: v1alpha1.AegisAlertStatus{
				Conditions: []v1alpha1.AlertOpsCondition{{
					Type:   tc.conditionType,
					Status: tc.conditionStatus,
				}},
			},
		}

		if tc.expectAlertNotFinished == IsAlertOpsFinished(alert) {
			if tc.expectAlertNotFinished {
				t.Errorf("test name: %s, alert ops was not expected to be finished", name)
			} else {
				t.Errorf("test name: %s, alert ops was expected to be finished", name)
			}
		}
	}
}
