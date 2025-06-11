package diagnosis

import (
	v1alpha1 "github.com/scitix/aegis/pkg/apis/diagnosis/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IsDiagnoseFinished checks whether the given diagnosis's has finished execution.
func IsDiagnoseFinished(diagnosis *v1alpha1.AegisDiagnosis) bool {
	if diagnosis.Status.ErrorResult != nil || diagnosis.Status.Result != nil {
		return true
	}
	return false
}

func IsDiagnoseSucceed(diagnosis *v1alpha1.AegisDiagnosis) bool {
	if diagnosis.Status.Result != nil {
		return true
	}
	return false
}

func IsDiagnoseFailed(diagnosis *v1alpha1.AegisDiagnosis) bool {
	if diagnosis.Status.ErrorResult != nil {
		return true
	}
	return false
}

func CheckDiagnoseExpireTTL(diagnosis *v1alpha1.AegisDiagnosis) (expired bool, ttl int32) {
	now := metav1.Now()
	start := diagnosis.CreationTimestamp.Time

	ttlStrategy := diagnosis.Spec.DeepCopy().TTLStrategy
	if ttlStrategy == nil {
		var d7 int32 = 7 * 24 * 3600
		ttlStrategy = &v1alpha1.TTLStrategy{
			SecondsAfterCompletion: &d7,
		}
	}

	if ttlStrategy.SecondsAfterCompletion != nil && IsDiagnoseFinished(diagnosis) {
		return true, *ttlStrategy.SecondsAfterCompletion - int32(now.Sub(start).Seconds())
	} else if ttlStrategy.SecondsAfterSuccess != nil && IsDiagnoseSucceed(diagnosis) {
		return true, *ttlStrategy.SecondsAfterSuccess - int32(now.Sub(start).Seconds())
	} else if ttlStrategy.SecondsAfterFailure != nil && IsDiagnoseFailed(diagnosis) {
		return true, *ttlStrategy.SecondsAfterFailure - int32(now.Sub(start).Seconds())
	} else {
		return false, 0
	}
}
