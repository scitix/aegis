package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	nodecheckv1alpha1 "github.com/scitix/aegis/pkg/apis/nodecheck/v1alpha1"

	alertv1alpha1 "github.com/scitix/aegis/pkg/apis/alert/v1alpha1"
	alertclientset "github.com/scitix/aegis/pkg/generated/alert/clientset/versioned"
	alertlister "github.com/scitix/aegis/pkg/generated/alert/listers/alert/v1alpha1"

	wfv1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type ResyncPeriodFunc func() time.Duration

// Returns 0 for resyncPeriod in case resyncing is not needed.
func NoResyncPeriodFunc() time.Duration {
	return 0
}

// StaticResyncPeriodFunc returns the resync period specified
func StaticResyncPeriodFunc(resyncPeriod time.Duration) ResyncPeriodFunc {
	return func() time.Duration {
		return resyncPeriod
	}
}

const (
	FailedCreateAlertReason      = "FailedCreate"
	FailedPatchAlertReason       = "FailedPatch"
	FailedDeleteAlertReason      = "FailedDelete"
	SucceessfulDeleteAlertReason = "SuccessfulDelete"
)

type AlertControllerInterface interface {
	CreateAlert(ctx context.Context, namespace string, template *alertv1alpha1.AegisAlert) error
	CreateAlertWithGenerateName(ctx context.Context, namespace string, template *alertv1alpha1.AegisAlert, generateName string) error
	ListAlertWithLabelSelector(ctx context.Context, namespace string, labelSelector labels.Selector) ([]*alertv1alpha1.AegisAlert, error)
	PatchAlert(ctx context.Context, namespace string, name string, data []byte) error
	PatchAlertWithLabelSelector(ctx context.Context, namespace string, labelSelector labels.Selector, data []byte) error
	DeleteAlert(ctx context.Context, namespace string, name string) error
}

type RealAlertController struct {
	AlertClient alertclientset.Interface
	AlertLister alertlister.AegisAlertLister
}

func (r *RealAlertController) CreateAlert(ctx context.Context, namespace string, template *alertv1alpha1.AegisAlert) error {
	return r.CreateAlertWithGenerateName(ctx, namespace, template, "")
}

func (r *RealAlertController) ListAlertWithLabelSelector(ctx context.Context, namespace string, selector labels.Selector) ([]*alertv1alpha1.AegisAlert, error) {
	return r.AlertLister.AegisAlerts(namespace).List(selector)
}

func (r *RealAlertController) CreateAlertWithGenerateName(ctx context.Context, namespace string, template *alertv1alpha1.AegisAlert, generateName string) error {
	if len(generateName) > 0 {
		template.GenerateName = generateName
	}

	_, err := r.AlertClient.AegisV1alpha1().AegisAlerts(namespace).Create(ctx, template, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (r *RealAlertController) PatchAlert(ctx context.Context, namespace string, name string, data []byte) error {
	_, err := r.AlertClient.AegisV1alpha1().AegisAlerts(namespace).Patch(ctx, name, types.JSONPatchType, data, metav1.PatchOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (r *RealAlertController) PatchAlertWithLabelSelector(ctx context.Context, namespace string, selector labels.Selector, data []byte) error {
	alerts, err := r.AlertLister.AegisAlerts(namespace).List(selector)
	if err != nil {
		return err
	}

	var errs []error
	for _, alert := range alerts {
		err := r.PatchAlert(ctx, namespace, alert.Name, data)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.NewAggregate(errs)
}

func (r *RealAlertController) DeleteAlert(ctx context.Context, namespace string, name string) error {
	return r.AlertClient.AegisV1alpha1().AegisAlerts(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// Reasons for workflow events
const (
	FailedCreateWorkflowReason      = "FailedCreate"
	FailedDeleteWorkflowReason      = "FailedDelete"
	SucceessfulDeleteWorkflowReason = "SuccessfulDelete"
	SuccessfulCreateWorfklowReason  = "SuccessfulCreate"
)

type WorkflowControllerInterface interface {
	CreateWorkflowWithPlainContent(ctx context.Context, namespace string, template string, object runtime.Object, controllerRef *metav1.OwnerReference) error

	CreateWorkflow(ctx context.Context, namespace string, template *wfv1alpha1.Workflow, object runtime.Object, controllerRef *metav1.OwnerReference) error

	CreateWorkflowWithGenerateName(ctx context.Context, namespace string, template *wfv1alpha1.Workflow, object runtime.Object, controllerRef *metav1.OwnerReference, generateName string) error

	PatchWorkflow(ctx context.Context, namespace string, name string, data []byte) error

	DeleteWorkflow(ctx context.Context, namespace string, workflowID string, object runtime.Object) error
}

// implement for argo workflow controller
type RealWorkflowControl struct {
	WfClient wfclientset.Interface
	Recorder record.EventRecorder
}

var _ WorkflowControllerInterface = &RealWorkflowControl{}

func getWorkflowPrefix(controllerName string) string {
	prefix := fmt.Sprintf("%s-", controllerName)
	if len(validation.NameIsDNSSubdomain(prefix, true)) != 0 {
		prefix = controllerName
	}

	return prefix
}

func getWorkflowLabelSet(template *wfv1alpha1.Workflow) labels.Set {
	desiredLabels := make(labels.Set)
	for k, v := range template.Labels {
		desiredLabels[k] = v
	}

	return desiredLabels
}

func getWorkflowAnnotationSet(template *wfv1alpha1.Workflow) labels.Set {
	desiredAnnotations := make(labels.Set)
	for k, v := range template.Annotations {
		desiredAnnotations[k] = v
	}

	return desiredAnnotations
}

func getWorkflowFinalizers(template *wfv1alpha1.Workflow) []string {
	desiredFinalizers := make([]string, len(template.Finalizers))
	copy(desiredFinalizers, template.Finalizers)
	return desiredFinalizers
}

func validateControllerRef(controllerRef *metav1.OwnerReference) error {
	if controllerRef == nil {
		return fmt.Errorf("controllerRef is nil")
	}

	if len(controllerRef.APIVersion) == 0 {
		return fmt.Errorf("controllerRef has empty APIVersion")
	}

	if len(controllerRef.Kind) == 0 {
		return fmt.Errorf("controllerRef has empty Kind")
	}

	if controllerRef.Controller == nil || *controllerRef.Controller != true {
		return fmt.Errorf("controllerRef.Controller is not set to true")
	}

	if controllerRef.BlockOwnerDeletion == nil || *controllerRef.BlockOwnerDeletion != true {
		return fmt.Errorf("controllerRef.BlockOwnerDeletion is not set to true")
	}

	return nil
}

func (r RealWorkflowControl) CreateWorkflowWithPlainContent(ctx context.Context, namespace string, template string, object runtime.Object, controllerRef *metav1.OwnerReference) error {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(template), nil, nil)
	if err != nil {
		return err
	}
	return r.CreateWorkflow(ctx, namespace, obj.(*wfv1alpha1.Workflow), object, controllerRef)
}

func (r RealWorkflowControl) CreateWorkflow(ctx context.Context, namespace string, template *wfv1alpha1.Workflow, object runtime.Object, controllerRef *metav1.OwnerReference) error {
	return r.CreateWorkflowWithGenerateName(ctx, namespace, template, object, controllerRef, "")
}

func (r RealWorkflowControl) CreateWorkflowWithGenerateName(ctx context.Context, namespace string, template *wfv1alpha1.Workflow, object runtime.Object, controllerRef *metav1.OwnerReference, generateName string) error {
	if err := validateControllerRef(controllerRef); err != nil {
		return err
	}

	wf, err := GetWorkflowFromSpec(template, object, controllerRef)
	if err != nil {
		return err
	}

	if len(generateName) > 0 {
		wf.ObjectMeta.GenerateName = generateName
	}

	return r.createWorkflow(ctx, namespace, wf, object)
}

func GetWorkflowFromSpec(template *wfv1alpha1.Workflow, object runtime.Object, controllerRef *metav1.OwnerReference) (*wfv1alpha1.Workflow, error) {
	// desiredLables := getWorkflowLabelSet(template)
	desiredAnnotations := getWorkflowAnnotationSet(template)
	desiredFinalizers := getWorkflowFinalizers(template)
	accessor, err := meta.Accessor(object)
	if err != nil {
		return nil, fmt.Errorf("object does not have ObjectMeta, %v", err)
	}
	prefix := getWorkflowPrefix(accessor.GetName())
	desiredLables := accessor.GetLabels()
	// for key, value := range accessor.GetLabels() {
	// 	desiredLables[key] = value
	// }

	wf := &wfv1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       desiredLables,
			Annotations:  desiredAnnotations,
			GenerateName: prefix,
			Finalizers:   desiredFinalizers,
		},
	}

	if controllerRef != nil {
		wf.OwnerReferences = append(wf.OwnerReferences, *controllerRef)
	}

	wf.Spec = *template.Spec.DeepCopy()

	return wf, nil
}

func (r RealWorkflowControl) PatchWorkflow(ctx context.Context, namespace string, name string, data []byte) error {
	_, err := r.WfClient.ArgoprojV1alpha1().Workflows(namespace).Patch(ctx, name, types.StrategicMergePatchType, data, metav1.PatchOptions{})
	return err
}

func (r RealWorkflowControl) DeleteWorkflow(ctx context.Context, namespace string, workflowID string, object runtime.Object) error {
	accessor, err := meta.Accessor(object)
	if err != nil {
		return fmt.Errorf("object dose not have objectMeta. %v", err)
	}

	klog.V(2).InfoS("Controller %v deleting workflow %s/%s", accessor.GetName(), namespace, workflowID)
	if err := r.WfClient.ArgoprojV1alpha1().Workflows(namespace).Delete(ctx, workflowID, metav1.DeleteOptions{}); err != nil {
		r.Recorder.Eventf(object, v1.EventTypeWarning, FailedDeleteWorkflowReason, "Error deleting: %v", err)
		return fmt.Errorf("unable to delete workflow: %v", err)
	}

	r.Recorder.Eventf(object, v1.EventTypeNormal, SucceessfulDeleteWorkflowReason, "Deleted workflow: %v", workflowID)
	return nil
}

func (r RealWorkflowControl) createWorkflow(ctx context.Context, namespace string, workflow *wfv1alpha1.Workflow, object runtime.Object) error {
	newWorkflow, err := r.WfClient.ArgoprojV1alpha1().Workflows(namespace).Create(ctx, workflow, metav1.CreateOptions{})
	if err != nil {
		if apierrors.HasStatusCause(err, v1.NamespaceTerminatingCause) {
			r.Recorder.Eventf(object, v1.EventTypeWarning, FailedCreateWorkflowReason, "Error creating: %v", err)
		}
		return err
	}

	accessor, err := meta.Accessor(object)
	if err != nil {
		return fmt.Errorf("object does not have ObjectMeta, %v", err)
	}
	klog.V(4).Infof("Controller %s created worfklow %v", accessor.GetName(), newWorkflow.Name)
	r.Recorder.Eventf(object, v1.EventTypeNormal, SuccessfulCreateWorfklowReason, "Create workflow: %v", workflow.Name)
	return nil
}

type FakeWorkflowControl struct {
	sync.Mutex
}

func FilterActiveWorkflow(workflows []*wfv1alpha1.Workflow) []*wfv1alpha1.Workflow {
	var result []*wfv1alpha1.Workflow
	for _, w := range workflows {
		if IsWorkflowActive(w) {
			result = append(result, w)
		} else {
			klog.V(4).Infof("Ignoring inactive workflow %v/%v in state %v, deletion time %v",
				w.Namespace, w.Name, w.Status.Phase, w.DeletionTimestamp)
		}
	}
	return result
}

func FilterSucceededWorkflow(workflows []*wfv1alpha1.Workflow) []*wfv1alpha1.Workflow {
	var result []*wfv1alpha1.Workflow
	for _, w := range workflows {
		if IsWorkflowSucceeded(w) {
			result = append(result, w)
		}
	}
	return result
}

func FilterFailedWorkflow(workflows []*wfv1alpha1.Workflow) []*wfv1alpha1.Workflow {
	var result []*wfv1alpha1.Workflow
	for _, w := range workflows {
		if IsWorkflowFailed(w) {
			result = append(result, w)
		}
	}
	return result
}

func IsWorkflowActive(wf *wfv1alpha1.Workflow) bool {
	return !wf.Status.Phase.Completed() && wf.DeletionTimestamp == nil
}

func IsWorkflowSucceeded(wf *wfv1alpha1.Workflow) bool {
	return wf.Status.Phase == wfv1alpha1.WorkflowSucceeded
}

func IsWorkflowFailed(wf *wfv1alpha1.Workflow) bool {
	return wf.Status.Phase == wfv1alpha1.WorkflowError || wf.Status.Phase == wfv1alpha1.WorkflowFailed
}

func FilterActiveNodeCheck(ncs []*nodecheckv1alpha1.AegisNodeHealthCheck) []*nodecheckv1alpha1.AegisNodeHealthCheck {
	var result []*nodecheckv1alpha1.AegisNodeHealthCheck
	for _, w := range ncs {
		if IsNodeCheckActive(w) {
			result = append(result, w)
		} else {
			klog.V(4).Infof("Ignoring inactive nodecheck %v/%v in state %v", w.Namespace, w.Name, w.Status.Status)
		}
	}
	return result
}

func FilterSucceededNodeCheck(ncs []*nodecheckv1alpha1.AegisNodeHealthCheck) []*nodecheckv1alpha1.AegisNodeHealthCheck {
	var result []*nodecheckv1alpha1.AegisNodeHealthCheck
	for _, w := range ncs {
		if IsNodeCheckSucceeded(w) {
			result = append(result, w)
		}
	}
	return result
}

func FilterFailedNodeCheck(ncs []*nodecheckv1alpha1.AegisNodeHealthCheck) []*nodecheckv1alpha1.AegisNodeHealthCheck {
	var result []*nodecheckv1alpha1.AegisNodeHealthCheck
	for _, w := range ncs {
		if IsNodeCheckFailed(w) {
			result = append(result, w)
		}
	}
	return result
}

func IsNodeCheckActive(nc *nodecheckv1alpha1.AegisNodeHealthCheck) bool {
	return !IsNodeCheckFailed(nc) && !IsNodeCheckSucceeded(nc)
}

func IsNodeCheckSucceeded(nc *nodecheckv1alpha1.AegisNodeHealthCheck) bool {
	return nc.Status.Status == nodecheckv1alpha1.CheckStatusSucceeded
}

func IsNodeCheckFailed(nc *nodecheckv1alpha1.AegisNodeHealthCheck) bool {
	return nc.Status.Status == nodecheckv1alpha1.CheckStatusFailed
}

// AegisCallbackInterface define aegis crd lifecycle callback
type AegisCallbackInterface interface {
	OnCreate(alert *alertv1alpha1.AegisAlert) error
	OnUpdate(alert *alertv1alpha1.AegisAlert) error
	OnDelete(alert *alertv1alpha1.AegisAlert) error
	OnNoOpsRule(alert *alertv1alpha1.AegisAlert) error
	OnNoOpsTemplate(alert *alertv1alpha1.AegisAlert) error
	OnFailedCreateOpsWorkflow(alert *alertv1alpha1.AegisAlert) error
	OnSucceedCreateOpsWorkflow(alert *alertv1alpha1.AegisAlert) error
	OnOpsWorkflowSucceed(alert *alertv1alpha1.AegisAlert) error
	OnOpsWorkflowFailed(alert *alertv1alpha1.AegisAlert) error
	OnNodeCheckUpdate(nodecheck *nodecheckv1alpha1.AegisNodeHealthCheck) error
}
