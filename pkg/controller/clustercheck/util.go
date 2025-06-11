package clustercheck

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/scitix/aegis/pkg/apis/clustercheck/v1alpha1"
)

func IsClusterCheckFinsihed(check *v1alpha1.AegisClusterHealthCheck) bool {
	for _, c := range check.Status.Conditions {
		if (c.Type == v1alpha1.CheckCompleted || c.Type == v1alpha1.CheckFailedCreateNodeCheck) && c.Status == v1.ConditionTrue {
			return true
		}
	}

	return false
}

func IsClusterCheckSucceeded(check *v1alpha1.AegisClusterHealthCheck) bool {
	for _, c := range check.Status.Conditions {
		if c.Type == v1alpha1.CheckCompleted && c.Status == v1.ConditionTrue {
			return true
		}
	}

	return false
}

func IsClusterCheckFailed(check *v1alpha1.AegisClusterHealthCheck) bool {
	for _, c := range check.Status.Conditions {
		if (c.Type == v1alpha1.CheckFailedCreateNodeCheck) && c.Status == v1.ConditionTrue {
			return true
		}
	}

	return false
}

func IsClusterCheckTimeout(check *v1alpha1.AegisClusterHealthCheck) bool {
	if check.Spec.Timeout == nil {
		return false
	}

	now := metav1.Now()
	start := check.Status.StartTime.Time

	return now.Sub(start).Seconds() > float64(*check.Spec.Timeout)
}

func ToleratesTaint(toleration v1.Toleration, taint []v1.Taint) bool {
	for _, t := range taint {
		if !toleration.ToleratesTaint(&t) {
			return false
		}
	}
	return true
}

func getNextScheduleTime(check *v1alpha1.AegisClusterHealthCheck, now time.Time, schedule cron.Schedule, recorder record.EventRecorder) (*time.Time, error) {
	var (
		earliestTime time.Time
	)
	if check.Status.StartTime != nil {
		earliestTime = check.Status.StartTime.Time
	} else {
		earliestTime = check.ObjectMeta.CreationTimestamp.Time
	}

	if earliestTime.After(now) {
		return nil, nil
	}

	t, numberOfMissedSchedules, err := getMostRecentScheduleTime(earliestTime, now, schedule)

	if numberOfMissedSchedules > 100 {
		// An object might miss several starts. For example, if
		// controller gets wedged on friday at 5:01pm when everyone has
		// gone home, and someone comes in on tuesday AM and discovers
		// the problem and restarts the controller, then all the hourly
		// jobs, more than 80 of them for one hourly cronJob, should
		// all start running with no further intervention (if the cronJob
		// allows concurrency and late starts).
		//
		// However, if there is a bug somewhere, or incorrect clock
		// on controller's server or apiservers (for setting creationTimestamp)
		// then there could be so many missed start times (it could be off
		// by decades or more), that it would eat up all the CPU and memory
		// of this controller. In that case, we want to not try to list
		// all the missed start times.
		//
		// I've somewhat arbitrarily picked 100, as more than 80,
		// but less than "lots".
		recorder.Eventf(check, v1.EventTypeWarning, "TooManyMissedTimes", "too many missed start times: %d. Set or decrease .spec.startingDeadlineSeconds or check clock skew", numberOfMissedSchedules)
		klog.InfoS("too many missed times", "cronjob", klog.KRef(check.GetNamespace(), check.GetName()), "missed times", numberOfMissedSchedules)
	}
	return t, err
}

func getMostRecentScheduleTime(earliestTime time.Time, now time.Time, schedule cron.Schedule) (*time.Time, int64, error) {
	t1 := schedule.Next(earliestTime)
	t2 := schedule.Next(t1)

	if now.Before(t1) {
		return nil, 0, nil
	}
	if now.Before(t2) {
		return &t1, 1, nil
	}

	// It is possible for cron.ParseStandard("59 23 31 2 *") to return an invalid schedule
	// seconds - 59, minute - 23, hour - 31 (?!)  dom - 2, and dow is optional, clearly 31 is invalid
	// In this case the timeBetweenTwoSchedules will be 0, and we error out the invalid schedule
	timeBetweenTwoSchedules := int64(t2.Sub(t1).Round(time.Second).Seconds())
	if timeBetweenTwoSchedules < 1 {
		return nil, 0, fmt.Errorf("time difference between two schedules less than 1 second")
	}
	timeElapsed := int64(now.Sub(t1).Seconds())
	numberOfMissedSchedules := (timeElapsed / timeBetweenTwoSchedules) + 1
	t := time.Unix(t1.Unix()+((numberOfMissedSchedules-1)*timeBetweenTwoSchedules), 0).UTC()
	return &t, numberOfMissedSchedules, nil
}
