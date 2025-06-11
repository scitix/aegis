package alert

import (
	v1alpha1 "github.com/scitix/aegis/pkg/apis/alert/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IsAlertOpsFinished checks whether the given alert's corresponding ops has finished execution.
func IsAlertOpsFinished(alert *v1alpha1.AegisAlert) bool {
	for _, c := range alert.Status.Conditions {
		if (c.Type == v1alpha1.AlertCompleteOpsWrofklow || c.Type == v1alpha1.AlertFailedOpsWrofklow || c.Type == v1alpha1.AlertFailedCreateOpsWorkflow) && c.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func IsAlertOpsSucceed(alert *v1alpha1.AegisAlert) bool {
	for _, c := range alert.Status.Conditions {
		if (c.Type == v1alpha1.AlertCompleteOpsWrofklow) && c.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func IsAlertOpsFailed(alert *v1alpha1.AegisAlert) bool {
	for _, c := range alert.Status.Conditions {
		if (c.Type == v1alpha1.AlertFailedOpsWrofklow) && c.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func CheckAlertExpireTTL(alert *v1alpha1.AegisAlert) (expired bool, ttl int32) {
	if alert.Spec.TTLStrategy == nil || alert.Status.StartTime == nil {
		return false, 0
	}

	now := metav1.Now()
	start := alert.Status.StartTime.Time
	for _, c := range alert.Status.Conditions {
		if c.Type == v1alpha1.AlertCompleteOpsWrofklow && c.Status == v1.ConditionTrue && alert.Spec.TTLStrategy.SecondsAfterSuccess != nil {
			ttl := *alert.Spec.TTLStrategy.SecondsAfterSuccess
			return true, ttl - int32(now.Sub(start).Seconds())
		}

		if c.Type == v1alpha1.AlertFailedOpsWrofklow && c.Status == v1.ConditionTrue && alert.Spec.TTLStrategy.SecondsAfterFailure != nil {
			ttl := *alert.Spec.TTLStrategy.SecondsAfterFailure
			return true, ttl - int32(now.Sub(start).Seconds())
		}

		if c.Type == v1alpha1.AlertFailedCreateOpsWorkflow && c.Status == v1.ConditionTrue && alert.Spec.TTLStrategy.SecondsAfterNoOps != nil {
			ttl := *alert.Spec.TTLStrategy.SecondsAfterNoOps
			return true, ttl - int32(now.Sub(start).Seconds())
		}
	}

	return false, 0
}
