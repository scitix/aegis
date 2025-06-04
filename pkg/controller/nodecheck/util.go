package nodecheck

import (
	v1 "k8s.io/api/core/v1"

	"gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/nodecheck/v1alpha1"
)

func IsNodeCheckFinished(nodecheck *v1alpha1.AegisNodeHealthCheck) bool {
	for _, c := range nodecheck.Status.Conditions {
		if (c.Type == v1alpha1.CheckSucceed || c.Type == v1alpha1.CheckFailed || c.Type == v1alpha1.CheckFailedCreatePod || c.Type == v1alpha1.CheckFailedFetchResult) && c.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func IsNodeCheckSucceeded(nodecheck *v1alpha1.AegisNodeHealthCheck) bool {
	for _, c := range nodecheck.Status.Conditions {
		if (c.Type == v1alpha1.CheckSucceed) && c.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func IsNodeCheckFailed(nodecheck *v1alpha1.AegisNodeHealthCheck) bool {
	for _, c := range nodecheck.Status.Conditions {
		if (c.Type == v1alpha1.CheckFailed || c.Type == v1alpha1.CheckFailedCreatePod || c.Type == v1alpha1.CheckFailedFetchResult) && c.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}
