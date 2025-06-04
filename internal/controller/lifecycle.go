package controller

import (
	"k8s.io/apimachinery/pkg/util/errors"

	alertv1alpha1 "gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/alert/v1alpha1"
	nodecheckv1apha1 "gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/nodecheck/v1alpha1"
	"gitlab.scitix-inner.ai/k8s/aegis/pkg/controller"
	"k8s.io/klog/v2"
)

type lifecycle struct {
	callbacks map[string]controller.AegisCallbackInterface
}

func newLifeCycle() *lifecycle {
	return &lifecycle{
		callbacks: make(map[string]controller.AegisCallbackInterface),
	}
}

func (l *lifecycle) register(name string, callback controller.AegisCallbackInterface) {
	if _, ok := l.callbacks[name]; !ok {
		l.callbacks[name] = callback
		klog.Infof("Succeeded register callback %s", name)
	} else {
		klog.Errorf("Failed to register existed callback %s", name)
	}
}

func (l *lifecycle) OnCreate(alert *alertv1alpha1.AegisAlert) error {
	errs := make([]error, 0)
	for _, callback := range l.callbacks {
		err := callback.OnCreate(alert)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.NewAggregate(errs)
}

func (l *lifecycle) OnUpdate(alert *alertv1alpha1.AegisAlert) error {
	errs := make([]error, 0)
	for _, callback := range l.callbacks {
		err := callback.OnUpdate(alert)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.NewAggregate(errs)
}

func (l *lifecycle) OnDelete(alert *alertv1alpha1.AegisAlert) error {
	errs := make([]error, 0)
	for _, callback := range l.callbacks {
		err := callback.OnDelete(alert)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.NewAggregate(errs)
}

func (l *lifecycle) OnNoOpsRule(alert *alertv1alpha1.AegisAlert) error {
	errs := make([]error, 0)
	for _, callback := range l.callbacks {
		err := callback.OnNoOpsRule(alert)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.NewAggregate(errs)
}

func (l *lifecycle) OnNoOpsTemplate(alert *alertv1alpha1.AegisAlert) error {
	errs := make([]error, 0)
	for _, callback := range l.callbacks {
		err := callback.OnNoOpsTemplate(alert)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.NewAggregate(errs)
}

func (l *lifecycle) OnFailedCreateOpsWorkflow(alert *alertv1alpha1.AegisAlert) error {
	errs := make([]error, 0)
	for _, callback := range l.callbacks {
		err := callback.OnFailedCreateOpsWorkflow(alert)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.NewAggregate(errs)
}

func (l *lifecycle) OnSucceedCreateOpsWorkflow(alert *alertv1alpha1.AegisAlert) error {
	errs := make([]error, 0)
	for _, callback := range l.callbacks {
		err := callback.OnSucceedCreateOpsWorkflow(alert)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.NewAggregate(errs)
}

func (l *lifecycle) OnOpsWorkflowSucceed(alert *alertv1alpha1.AegisAlert) error {
	errs := make([]error, 0)
	for _, callback := range l.callbacks {
		err := callback.OnOpsWorkflowSucceed(alert)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.NewAggregate(errs)
}

func (l *lifecycle) OnOpsWorkflowFailed(alert *alertv1alpha1.AegisAlert) error {
	errs := make([]error, 0)
	for _, callback := range l.callbacks {
		err := callback.OnOpsWorkflowFailed(alert)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.NewAggregate(errs)
}

func (l *lifecycle) OnNodeCheckUpdate(nodecheck *nodecheckv1apha1.AegisNodeHealthCheck) error {
	errs := make([]error, 0)
	for _, callback := range l.callbacks {
		err := callback.OnNodeCheckUpdate(nodecheck)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.NewAggregate(errs)
}
