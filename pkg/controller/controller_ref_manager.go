package controller

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	wfv1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	nativecontroller "k8s.io/kubernetes/pkg/controller"
)

type WorkflowControllerRefManager struct {
	nativecontroller.BaseControllerRefManager
	controllerKind     schema.GroupVersionKind
	workflowController WorkflowControllerInterface
	finalizers         []string
}

func NewWorkflowControllerRefManager(workflowController WorkflowControllerInterface,
	controller metav1.Object,
	selector labels.Selector,
	controllerKind schema.GroupVersionKind,
	canAdopt func(context.Context) error,
	finalizers ...string) *WorkflowControllerRefManager {
	return &WorkflowControllerRefManager{
		BaseControllerRefManager: nativecontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		controllerKind:     controllerKind,
		workflowController: workflowController,
		finalizers:         finalizers,
	}
}

// ClaimWorkflows tries to take ownership of a list of Workflows
func (m *WorkflowControllerRefManager) ClaimWorkflows(ctx context.Context, workflows []*wfv1alpha1.Workflow, filters ...func(*wfv1alpha1.Workflow) bool) ([]*wfv1alpha1.Workflow, error) {
	var claimed []*wfv1alpha1.Workflow
	var errlist []error

	match := func(obj metav1.Object) bool {
		workflow := obj.(*wfv1alpha1.Workflow)
		if !m.Selector.Matches(labels.Set(workflow.Labels)) {
			return false
		}

		for _, filter := range filters {
			if !(filter(workflow)) {
				return false
			}
		}
		return true
	}
	adopted := func(ctx context.Context, obj metav1.Object) error {
		return m.AdoptWorkflow(ctx, obj.(*wfv1alpha1.Workflow))
	}
	release := func(ctx context.Context, obj metav1.Object) error {
		return m.ReleasedWorkflow(ctx, obj.(*wfv1alpha1.Workflow))
	}

	for _, workflow := range workflows {
		ok, err := m.ClaimObject(ctx, workflow, match, adopted, release)
		if err != nil {
			errlist = append(errlist, err)
			continue
		}
		if ok {
			claimed = append(claimed, workflow)
		}
	}
	return claimed, utilerrors.NewAggregate(errlist)
}

// AdoptWorkflow send a patch to take control of the workflow
func (m *WorkflowControllerRefManager) AdoptWorkflow(ctx context.Context, workflow *wfv1alpha1.Workflow) error {
	if err := m.CanAdopt(ctx); err != nil {
		return fmt.Errorf("can't adopted Pod %v/%v (%v): %v", workflow.Namespace, workflow.Name, workflow.UID, err)
	}

	patchBytes, err := ownerRefControllerPatch(m.Controller, m.controllerKind, workflow.UID, m.finalizers...)
	if err != nil {
		return err
	}
	return m.workflowController.PatchWorkflow(ctx, workflow.Namespace, workflow.Name, patchBytes)
}

// ReleasedWorkflow free the workflow from the control of the controller
func (m *WorkflowControllerRefManager) ReleasedWorkflow(ctx context.Context, workflow *wfv1alpha1.Workflow) error {
	klog.V(2).Infof("patching workflow %s_%s to remove its controllerRef to %s/%s:%s",
		workflow.Namespace, workflow.Name, m.controllerKind.GroupVersion(), m.controllerKind.Kind, m.Controller.GetName())
	patchBytes, err := GenerateDeleteOwnerRefStrategicMergeBytes(workflow.UID, []types.UID{m.Controller.GetUID()}, m.finalizers...)
	if err != nil {
		return err
	}
	err = m.workflowController.PatchWorkflow(ctx, workflow.Namespace, workflow.Name, patchBytes)
	if err != nil {
		if errors.IsNotFound(err) {
			// If the pod no longer exists, ignore it.
			return nil
		}
		if errors.IsInvalid(err) {
			return nil
		}
	}
	return err
}

type objectForAddOwnerRefPatch struct {
	Metadata objectMetaForPatch `json:"metadata"`
}

type objectMetaForPatch struct {
	OwnerReferences []metav1.OwnerReference `json:"ownerReferences"`
	UID             types.UID               `json:"uid"`
	Finalizers      []string                `json:"finalizers,omitempty"`
}

func ownerRefControllerPatch(controller metav1.Object, controllerKind schema.GroupVersionKind, uid types.UID, finalizers ...string) ([]byte, error) {
	blockOwnerDeletion := true
	isController := true
	addControllerPatch := objectForAddOwnerRefPatch{
		Metadata: objectMetaForPatch{
			UID: uid,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         controllerKind.GroupVersion().String(),
					Kind:               controllerKind.Kind,
					Name:               controller.GetName(),
					UID:                controller.GetUID(),
					Controller:         &isController,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
			Finalizers: finalizers,
		},
	}
	patchBytes, err := json.Marshal(&addControllerPatch)
	if err != nil {
		return nil, err
	}
	return patchBytes, nil
}

type objectForDeleteOwnerRefStrategicMergePatch struct {
	Metadata objectMetaForMergePatch `json:"metadata"`
}

type objectMetaForMergePatch struct {
	UID              types.UID           `json:"uid"`
	OwnerReferences  []map[string]string `json:"ownerReferences"`
	DeleteFinalizers []string            `json:"$deleteFromPrimitiveList/finalizers,omitempty"`
}

func GenerateDeleteOwnerRefStrategicMergeBytes(dependentUID types.UID, ownerUIDs []types.UID, finalizers ...string) ([]byte, error) {
	var ownerReferences []map[string]string
	for _, ownerUID := range ownerUIDs {
		ownerReferences = append(ownerReferences, ownerReference(ownerUID, "delete"))
	}
	patch := objectForDeleteOwnerRefStrategicMergePatch{
		Metadata: objectMetaForMergePatch{
			UID:              dependentUID,
			OwnerReferences:  ownerReferences,
			DeleteFinalizers: finalizers,
		},
	}
	patchBytes, err := json.Marshal(&patch)
	if err != nil {
		return nil, err
	}
	return patchBytes, nil
}

func ownerReference(uid types.UID, patchType string) map[string]string {
	return map[string]string{
		"$patch": patchType,
		"uid":    string(uid),
	}
}
