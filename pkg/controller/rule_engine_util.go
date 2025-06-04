package controller

import (
	corev1 "k8s.io/api/core/v1"
)

type Condition struct {
	Type   string
	Status string
}

type MatchRule struct {
	Labels    map[string]string // match all
	Condition *Condition        // match any
}

type RuleEngineInterface interface {
	GetTemplateRefs(r *MatchRule) ([]*corev1.ObjectReference, error)

	GetTemplateContentByRefs(ref *corev1.ObjectReference) (string, error)

	SucceedExecuteTemplateCallback(ref *corev1.ObjectReference)

	FailedExecuteTemplateCallback(ref *corev1.ObjectReference)
}
