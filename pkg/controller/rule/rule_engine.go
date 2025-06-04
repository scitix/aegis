package rule

import (
	"context"
	"fmt"
	"regexp"

	ruleapi "gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/rule/v1alpha1"
	templatev1alpha1 "gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/template/v1alpha1"
	"gitlab.scitix-inner.ai/k8s/aegis/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

var templateControllerKind = templatev1alpha1.SchemeGroupVersion.WithKind("AegisOpsTemplate")

func matchCondition(condition *controller.Condition, targets []ruleapi.AegisAlertCondition) bool {
	if condition == nil {
		return false
	}
	for _, con := range targets {
		if con.Status != condition.Status {
			continue
		}

		if match, _ := regexp.MatchString(con.Type, condition.Type); !match {
			continue
		}

		return true
	}

	return false
}

func (c *RuleController) GetTemplateRefs(r *controller.MatchRule) ([]*corev1.ObjectReference, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// selector := &metav1.LabelSelector{
	// 	MatchLabels:      make(map[string]string),
	// 	MatchExpressions: make([]metav1.LabelSelectorRequirement, 0),
	// }

	// err := metav1.Convert_Map_string_To_string_To_v1_LabelSelector(&r.Labels, selector, nil)
	// if err != nil {
	// 	return nil, fmt.Errorf("fail to convert rule labels(%+v) to LabelSelector: %v", r.Labels, err)
	// }

	// labels, err := metav1.LabelSelectorAsSelector(selector)
	// if err != nil {
	// 	return nil, fmt.Errorf("fail to convert selector(%v) to labels: %v", selector, err)
	// }
	refs := make([]*corev1.ObjectReference, 0)
	for key, rule := range c.ruleCache {
		selector, err := metav1.LabelSelectorAsSelector(rule.Spec.Selector)
		if err != nil {
			klog.Errorf("Error when deal with rule(%s) labelselector: %v, ignore", key, err)
		}

		if matchCondition(r.Condition, rule.Spec.AlertConditions) && (len(selector.String()) == 0 || selector.Matches(labels.Set(r.Labels))) {
			klog.V(6).Infof("rule %s match condition: %v", key, r)
			refs = append(refs, rule.Spec.OpsTemplate)
		} else {
			klog.V(6).Infof("rule %s don't match condition: %v", key, r)
		}
	}

	return refs, nil
}

func (c *RuleController) GetTemplateContentByRefs(ref *corev1.ObjectReference) (string, error) {
	if ref.Kind != templateControllerKind.Kind {
		return "", fmt.Errorf("controller kind dismatch, wanted: %s, got: %v", templateControllerKind.Kind, ref.Kind)
	}

	template, err := c.templateLister.AegisOpsTemplates(ref.Namespace).Get(ref.Name)
	if err != nil {
		return "", err
	}

	return template.Spec.Manifest, nil
}

func (c *RuleController) SucceedExecuteTemplateCallback(ref *corev1.ObjectReference) {
	c.mu.Lock()
	defer c.mu.Unlock()
	klog.V(6).Infof("increase template %s/%s succeed status field", ref.Namespace, ref.Name)

	template, err := c.templateLister.AegisOpsTemplates(ref.Namespace).Get(ref.Name)
	if err != nil {
		klog.Warningf("Get template %s/%s failed: %v", ref.Namespace, ref.Name, err)
		return
	}

	template.Status.ExecuteStatus.Succeeded = template.Status.ExecuteStatus.Succeeded + 1
	_, err = c.templateclientset.AegisV1alpha1().AegisOpsTemplates(ref.Namespace).Update(context.Background(), template, metav1.UpdateOptions{})
	if err != nil {
		klog.Warningf("Update template %s/%s failed: %v", ref.Namespace, ref.Name, err)
		return
	}
}

func (c *RuleController) FailedExecuteTemplateCallback(ref *corev1.ObjectReference) {
	c.mu.Lock()
	defer c.mu.Unlock()
	klog.V(6).Infof("increase template %s/%s failed status field", ref.Namespace, ref.Name)

	template, err := c.templateLister.AegisOpsTemplates(ref.Namespace).Get(ref.Name)
	if err != nil {
		klog.Warningf("Get template %s/%s failed: %v", ref.Namespace, ref.Name, err)
		return
	}

	template.Status.ExecuteStatus.Failed = template.Status.ExecuteStatus.Failed + 1
	_, err = c.templateclientset.AegisV1alpha1().AegisOpsTemplates(ref.Namespace).Update(context.Background(), template, metav1.UpdateOptions{})
	if err != nil {
		klog.Warningf("Update template %s/%s failed: %v", ref.Namespace, ref.Name, err)
		return
	}
}
