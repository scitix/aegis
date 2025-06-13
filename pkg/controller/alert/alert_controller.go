package alert

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	alertv1alpha1 "github.com/scitix/aegis/pkg/apis/alert/v1alpha1"
	alertclientset "github.com/scitix/aegis/pkg/generated/alert/clientset/versioned"
	alertInformer "github.com/scitix/aegis/pkg/generated/alert/informers/externalversions/alert/v1alpha1"
	alertLister "github.com/scitix/aegis/pkg/generated/alert/listers/alert/v1alpha1"
	"github.com/scitix/aegis/tools"

	wfv1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	argoScheme "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/scheme"
	wfInformer "github.com/argoproj/argo-workflows/v3/pkg/client/informers/externalversions/workflow/v1alpha1"
	wfLister "github.com/argoproj/argo-workflows/v3/pkg/client/listers/workflow/v1alpha1"

	"github.com/scitix/aegis/pkg/controller"
	nativecontroller "k8s.io/kubernetes/pkg/controller"
)

func init() {
	alertv1alpha1.AddToScheme(scheme.Scheme)
	argoScheme.AddToScheme(scheme.Scheme)
}

const workflowDefaultUpdatePeriod = 500 * time.Millisecond
const alertDefaultFailurePeriod = 10 * time.Second

var controllerAgentName = "alert-Controller"

var controllerKind = alertv1alpha1.SchemeGroupVersion.WithKind("AegisAlert")

var callback func(callback func(alert *alertv1alpha1.AegisAlert) error, alert *alertv1alpha1.AegisAlert, alertKey string) = func(callback func(alert *alertv1alpha1.AegisAlert) error, alert *alertv1alpha1.AegisAlert, alertKey string) {
	name := runtime.FuncForPC(reflect.ValueOf(callback).Pointer()).Name()
	if err := callback(alert); err != nil {
		klog.Errorf("fail to execute lifecycle callback %v for alert %s: %v", name, alertKey, err)
	} else {
		klog.V(9).Infof("succeed execute lifecycle callback %v callback for alert %s", name, alertKey)
	}
}

// AlertController ensures all alert object has corresponding workflows
type AlertController struct {
	kubeClient      clientset.Interface
	workflowControl controller.WorkflowControllerInterface

	lifecycleControl controller.AegisCallbackInterface

	// TO allow injection of the following for testing
	updateStatusHandler func(ctx context.Context, alert *alertv1alpha1.AegisAlert) error
	// patchAlertHandler  func(ctx context.Context, alert *alertv1alpha1.AegisAlert, patch []byte) error
	syncHandler func(ctx context.Context, workflowKey string) (bool, error)

	// RuleEngineInterface: get ops refs according to alert condition
	ruleEngineController controller.RuleEngineInterface

	alertclientset alertclientset.Interface

	// A TTLCache of workflow create/delete each expect to use
	expectations nativecontroller.ControllerExpectationsInterface

	// a store for alerts
	alertLister alertLister.AegisAlertLister

	// a store for workflow controller
	workflowLister wfLister.WorkflowLister

	// alerts that need to be updated
	workqueue workqueue.RateLimitingInterface

	// Orphan deleted workflows that still have a tracking finalizer to be removed
	// orphanqueue workqueue.RateLimitingInterface

	broadcaster record.EventBroadcaster
	recorder    record.EventRecorder

	alertSynced    cache.InformerSynced
	workflowSynced cache.InformerSynced

	workflowUpdatePeriod time.Duration

	logger klog.Logger
}

// NewController create a new aegis alert controller
// kubeclient
// alertclient: alert resource controller
// workflowclient: argo workflow controller
// ruleEngineController: rule engine controller, for list correspending ops template
// wfinformer: argo workflow informer
// alertinformer: alert informer
func NewController(kubeclient kubernetes.Interface,
	alertclient alertclientset.Interface,
	workflowclient wfclientset.Interface,
	ruleEngineController controller.RuleEngineInterface,
	wfinformer wfInformer.WorkflowInformer,
	alertinformer alertInformer.AegisAlertInformer,
	lifecycleControl controller.AegisCallbackInterface) *AlertController {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: kubeclient.CoreV1().Events(v1.NamespaceAll)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: controllerAgentName})

	controller := &AlertController{
		kubeClient:       kubeclient,
		alertclientset:   alertclient,
		lifecycleControl: lifecycleControl,
		workflowControl: controller.RealWorkflowControl{
			WfClient: workflowclient,
			Recorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: controllerAgentName}),
		},
		expectations:         nativecontroller.NewControllerExpectations(),
		ruleEngineController: ruleEngineController,
		alertLister:          alertinformer.Lister(),
		workflowLister:       wfinformer.Lister(),
		workqueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "alerts"),
		// orphanqueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "alert_orphan_workflows"),
		broadcaster:          eventBroadcaster,
		recorder:             recorder,
		alertSynced:          alertinformer.Informer().HasSynced,
		workflowSynced:       wfinformer.Informer().HasSynced,
		workflowUpdatePeriod: workflowDefaultUpdatePeriod,
		logger:               klog.NewKlogr(),
	}

	klog.Info("Setting up event handles")

	alertinformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addAlert,
		UpdateFunc: controller.updateAlert,
		DeleteFunc: controller.deleteAlert,
	})

	wfinformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addWorkflow,
		UpdateFunc: controller.updateWorkflow,
		DeleteFunc: controller.deleteWorkflow,
	})

	controller.updateStatusHandler = controller.updateAlertStatus
	controller.syncHandler = controller.syncAlert

	return controller
}

func (c *AlertController) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.broadcaster.Shutdown()
	defer c.workqueue.ShutDown()

	klog.Info("Starting Alert controller")
	defer klog.Info("Shutting down Alert controller.")

	klog.Info("Waiting for alert informer caches to sync")
	if ok := cache.WaitForNamedCacheSync("alert", ctx.Done(), c.alertSynced, c.workflowSynced); !ok {
		return fmt.Errorf("failed to wait for cache to sync")
	}

	klog.Info("Starting alert workers")
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}
	// go wait.UntilWithContext(ctx, c.orphanWorkflow, time.Second)
	klog.Info("Started workers")

	<-ctx.Done()

	return nil
}

func (c *AlertController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *AlertController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	defer c.workqueue.Done(obj)

	forget, err := c.syncHandler(ctx, obj.(string))
	if err == nil {
		if forget {
			c.workqueue.Forget(obj)
		}
		return true
	}

	utilruntime.HandleError(fmt.Errorf("syncing alert: %s", err))
	if !apierrors.IsConflict(err) {
		c.workqueue.AddRateLimited(obj)
	}

	return true
}

// getWorkflowAAlerts return a list of alerts that potentially match a workflow
func (c *AlertController) getWorkflowAlerts(workflow *wfv1alpha1.Workflow) []*alertv1alpha1.AegisAlert {
	if len(workflow.Labels) == 0 {
		utilruntime.HandleError(fmt.Errorf("no alerts found for workflow %s/%v because it has no labels", workflow.Namespace, workflow.Name))
		return nil
	}

	var list []*alertv1alpha1.AegisAlert
	list, err := c.alertLister.AegisAlerts(workflow.Namespace).List(labels.Everything())
	if err != nil {
		return nil
	}

	var alerts []*alertv1alpha1.AegisAlert
	for _, alert := range list {
		selector, _ := metav1.LabelSelectorAsSelector(alert.Spec.Selector)
		if !selector.Matches(labels.Set(workflow.Labels)) {
			continue
		}
		alerts = append(alerts, alert)
	}
	if len(alerts) == 0 {
		utilruntime.HandleError(fmt.Errorf("could not find alerts for pod %s in namespace %s with labels: %v", workflow.Name, workflow.Namespace, workflow.Labels))
	}
	return alerts
}

// resolveControllerRef returns the controller referenced by a ControllerRef
func (c *AlertController) resolveControllerRef(namespace string, controllerRef *metav1.OwnerReference) *alertv1alpha1.AegisAlert {
	if controllerRef.Kind != controllerKind.Kind {
		return nil
	}

	alert, err := c.alertLister.AegisAlerts(namespace).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if alert.UID != controllerRef.UID {
		return nil
	}

	return alert
}

// when a workflow is created, enqueue the controller that manages it
func (c *AlertController) addWorkflow(obj interface{}) {
	workflow := obj.(*wfv1alpha1.Workflow)
	if workflow.DeletionTimestamp != nil {
		c.deleteWorkflow(workflow)
		return
	}

	if controllerRef := metav1.GetControllerOf(workflow); controllerRef != nil {
		alert := c.resolveControllerRef(workflow.Namespace, controllerRef)
		if alert == nil {
			return
		}

		alertKey, err := controller.KeyFunc(alert)
		if err != nil {
			return
		}

		c.expectations.CreationObserved(c.logger, alertKey)
		klog.V(4).Infof("enqueueing alert %s for workflow %s/%s added", alertKey, workflow.Namespace, workflow.Name)
		c.enqueueControllerWorkflowUpdate(alert, true)
		return
	}

	// for _, alert := range c.getWorkflowAlerts(workflow) {
	// 	c.enqueueController(alert)
	// }
}

// When a workflow is updated
func (c *AlertController) updateWorkflow(old, cur interface{}) {
	curWorkflow := cur.(*wfv1alpha1.Workflow)
	oldWorkflow := old.(*wfv1alpha1.Workflow)
	if curWorkflow.ResourceVersion == oldWorkflow.ResourceVersion {
		return
	}

	if curWorkflow.DeletionTimestamp != nil {
		c.deleteWorkflow(curWorkflow)
		return
	}

	immediate := (curWorkflow.Status.Phase != wfv1alpha1.WorkflowFailed) && (curWorkflow.Status.Phase != wfv1alpha1.WorkflowError)

	curControllerRef := metav1.GetControllerOf(curWorkflow)
	oldControllerRef := metav1.GetControllerOf(oldWorkflow)
	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	if controllerRefChanged && oldControllerRef != nil {
		// The ControllerRef was changed. Sync the old controller
		if alert := c.resolveControllerRef(oldWorkflow.Namespace, oldControllerRef); alert != nil {
			klog.V(4).Infof("enqueueing alert %s/%s for update workflow %s/%s controller ref", alert.Namespace, alert.Name, curWorkflow.Namespace, curWorkflow.Name)
			c.enqueueControllerWorkflowUpdate(alert, immediate)
		}
	}

	if curControllerRef != nil {
		alert := c.resolveControllerRef(curWorkflow.Namespace, curControllerRef)
		if alert == nil {
			return
		}

		alertKey, err := controller.KeyFunc(alert)
		if err != nil {
			return
		}

		klog.V(4).Infof("enqueueing alert %s for workflow %s/%s updated", alertKey, curWorkflow.Namespace, curWorkflow.Name)
		c.enqueueControllerWorkflowUpdate(alert, immediate)
		return
	}

	// labelChanged := !reflect.DeepEqual(curWorkflow.Labels, oldWorkflow.Labels)
	// if labelChanged || controllerRefChanged {
	// 	for _, alert := range c.getWorkflowAlerts(curWorkflow) {
	// 		c.enqueueController(alert)
	// 	}
	// }
}

// when a workflow is deleted. enqueue the alert that manages the workflow
func (c *AlertController) deleteWorkflow(obj interface{}) {
	workflow := obj.(*wfv1alpha1.Workflow)

	controllerRef := metav1.GetControllerOf(workflow)
	if controllerRef == nil {
		return
	}
	alert := c.resolveControllerRef(workflow.Namespace, controllerRef)
	if alert == nil || IsAlertOpsFinished(alert) {
		return
	}
	alertKey, err := controller.KeyFunc(alert)
	if err != nil {
		return
	}
	klog.V(4).Infof("enqueueing alert %s for workflow %s/%s deleted", alertKey, workflow.Namespace, workflow.Name)
	c.expectations.DeletionObserved(c.logger, alertKey)

	// c.enqueueControllerWorkflowUpdate(alert, true)
}

func (c *AlertController) addAlert(obj interface{}) {
	alert := obj.(*alertv1alpha1.AegisAlert)
	alertKey, err := controller.KeyFunc(alert)
	if err != nil {
		return
	}

	go callback(c.lifecycleControl.OnCreate, alert, alertKey)

	klog.Infof("enqueueing alert %s for added", alertKey)
	c.enqueueController(alert, true)
}

// ignore update
func (c *AlertController) updateAlert(old, cur interface{}) {
	oldAlert := old.(*alertv1alpha1.AegisAlert)
	curAlert := cur.(*alertv1alpha1.AegisAlert)
	if curAlert.ResourceVersion == oldAlert.ResourceVersion {
		return
	}

	// // never return error
	alertKey, err := controller.KeyFunc(curAlert)
	if err != nil {
		return
	}

	go callback(c.lifecycleControl.OnUpdate, curAlert, alertKey)

	klog.Infof("enqueueing alert %s for updated", alertKey)
	c.enqueueController(curAlert, true)

	// // TODO: add ttl
}

func (c *AlertController) deleteAlert(obj interface{}) {
	alert := obj.(*alertv1alpha1.AegisAlert)
	alertKey, err := controller.KeyFunc(alert)
	if err != nil {
		return
	}

	go callback(c.lifecycleControl.OnUpdate, alert, alertKey)

	klog.Infof("enqueueing alert %s for deleted", alertKey)
	c.enqueueController(obj, true)

	// Listing workflows shouldn't really fail, as we are just querying the informer cache.
	// selector, err := metav1.LabelSelectorAsSelector(alert.Spec.Selector)
	// if err != nil {
	// 	utilruntime.HandleError(fmt.Errorf("parsing deleted alert selector: %v", err))
	// 	return
	// }
	// workflows, _ := c.workflowLister.Workflows(alert.Namespace).List(selector)
	// for _, workflow := range workflows {
	// 	if metav1.IsControlledBy(workflow, alert) {
	// 		c.enqueueOrphanWorkflow(workflow)
	// 	}
	// }
}

func (c *AlertController) enqueueController(obj interface{}, immediate bool) {
	c.enqueueControllerDelayed(obj, immediate, 0)
}

func (c *AlertController) enqueueControllerWorkflowUpdate(obj interface{}, immediate bool) {
	c.enqueueControllerDelayed(obj, immediate, c.workflowUpdatePeriod)
}

func (c *AlertController) enqueueControllerDelayed(obj interface{}, immediate bool, delay time.Duration) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object: %v", obj, err))
	}

	c.workqueue.AddAfter(key, delay)
}

// func (c *AlertController) enqueueOrphanWorkflow(obj interface{}) {
// 	key, err := controller.KeyFunc(obj)
// 	if err != nil {
// 		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %+v: %v", obj, err))
// 		return
// 	}

// 	c.orphanqueue.Add(key)
// }

// func (c *AlertController) orphanWorkflow(ctx context.Context) {
// 	for c.processNexOrphanWorkflow(ctx) {

// 	}
// }

// func (c *AlertController) processNexOrphanWorkflow(ctx context.Context) bool {
// 	key, quit := c.orphanqueue.Get()
// 	if quit {
// 		return false
// 	}
// 	defer c.orphanqueue.Done(key)
// 	err := c.syncOrphanWorkflow(ctx, key.(string))
// 	if err != nil {
// 		utilruntime.HandleError(fmt.Errorf("Error syncing orphan workflow: %v", err))
// 		c.orphanqueue.AddRateLimited(key)
// 	} else {
// 		c.orphanqueue.Forget(key)
// 	}

// 	return true
// }

// syncOrphanWorkflow removes the tracking finalizer from an orphan workflow if found.
// func (c *AlertController) syncOrphanWorkflow(ctx context.Context, key string) error {
// 	startTime := time.Now()
// 	defer func() {
// 		klog.V(4).Infof("Finished syncing orphan workflow %q (%v)", key, time.Since(startTime))
// 	}()

// 	ns, name, err := cache.SplitMetaNamespaceKey(key)
// 	if err != nil {
// 		return err
// 	}

// 	sharedWorfklow, err := c.workflowLister.Workflows(ns).Get(name)
// 	if err != nil {
// 		if apierrors.IsNotFound(err) {
// 			klog.V(4).Infof("Orphan workflow has been deleted: %v", key)
// 			return nil
// 		}
// 	}

// 	if controllerRef := metav1.GetControllerOf(sharedWorfklow); controllerRef != nil {
// 		alert := c.resolveControllerRef(sharedWorfklow.Namespace, controllerRef)
// 		if alert != nil && !IsAlertOpsFinished(alert) {
// 			return nil
// 		}
// 	}

// 	return nil
// }

// getWorkflowsForAlert return the workflow set that belong to the alert
func (c *AlertController) getWorkflowsForAlert(ctx context.Context, alert *alertv1alpha1.AegisAlert, withFinalizer bool) ([]*wfv1alpha1.Workflow, error) {
	labelMap, err := metav1.LabelSelectorAsMap(alert.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("couldn't convert Alert selector: %v", err)
	}

	selector := labels.Set(labelMap).AsSelector()

	workflows, err := c.workflowLister.Workflows(alert.Namespace).List(selector)
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("List %d workflows for alert %s/%s", len(workflows), alert.Namespace, alert.Name)

	return workflows, nil
	// canAdoptFunc := nativecontroller.RecheckDeletionTimestamp(func() (metav1.Object, error) {
	// 	fresh, err := c.alertclientset.AegisV1alpha1().AegisAlerts(alert.Namespace).Get(ctx, alert.Name, metav1.GetOptions{})
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	if fresh.UID != alert.UID {
	// 		return nil, fmt.Errorf("original Alert %v/%v is gone: got uid %v, wanted %v", alert.Namespace, alert.Name, fresh.UID, alert.UID)
	// 	}
	// 	return fresh, nil
	// })

	// cm := controller.NewWorkflowControllerRefManager(c.workflowControl, alert, selector, controllerKind, canAdoptFunc)
	// return cm.ClaimWorkflows(ctx, workflows)
}

func (c *AlertController) syncAlert(ctx context.Context, key string) (forget bool, err error) {
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing alert %q (%v)", key, time.Since(startTime))
	}()

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return false, err
	}

	if len(ns) == 0 || len(name) == 0 {
		return false, fmt.Errorf("invalid alert key %q: either namespace or name is missing", key)
	}
	sharedAlert, err := c.alertLister.AegisAlerts(ns).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("Alert has been deleted: %v", key)
			c.expectations.DeleteExpectations(c.logger, key)
			return true, nil
		}
		return false, nil
	}

	alert := *sharedAlert.DeepCopy()

	// If alert finished previously. we don't want to redo the termination
	if IsAlertOpsFinished(&alert) {
		if IsAlertOpsSucceed(&alert) {
			go callback(c.lifecycleControl.OnOpsWorkflowSucceed, &alert, key)
		}

		if IsAlertOpsFailed(&alert) {
			go callback(c.lifecycleControl.OnOpsWorkflowFailed, &alert, key)
		}

		expired, ttl := CheckAlertExpireTTL(&alert)
		if expired && ttl > 0 {
			klog.V(4).Infof("Alert %v ttl second: %d", key, ttl)
			c.enqueueControllerDelayed(&alert, false, time.Duration(ttl)*time.Second)
		} else if expired {
			// cleanup alert
			klog.V(4).Infof("Alert %v ttl expired", key)
			if err := c.cleanAlert(ctx, &alert); err != nil {
				klog.V(4).ErrorS(err, "Failed to delete alert %v: %v", key, err)
				return false, err
			}
		}
		return true, nil
	}

	alertNeedSync := c.expectations.SatisfiedExpectations(c.logger, key)

	workflows, err := c.getWorkflowsForAlert(ctx, &alert, true)
	if err != nil {
		return false, nil
	}

	activeWorkflow, succeededWorkflow, failedWorkflow := controller.FilterActiveWorkflow(workflows), controller.FilterSucceededWorkflow(workflows), controller.FilterFailedWorkflow(workflows)
	active, succeeded, failed := int32(len(activeWorkflow)), int32(len(succeededWorkflow)), int32(len(failedWorkflow))
	total := int32(0)
	if alert.Status.OpsStatus.Total != nil {
		total = *alert.Status.OpsStatus.Total
	}

	if alert.Status.StartTime == nil {
		now := metav1.Now()
		alert.Status.StartTime = &now
		alert.Status.Status = string(alert.Spec.Status)
	}

	if total > 0 && alert.Status.OpsStatus.StartTime == nil {
		now := metav1.Now()
		alert.Status.OpsStatus.StartTime = &now
		alert.Status.OpsStatus.Status = alertv1alpha1.OpsStatusRunning
	}

	var createWorkflowErr error
	alertOpsFailed := false
	var failureReason string
	var failureMessage string

	// any failed lead to fail status
	if failed > 0 {
		alertOpsFailed = true
		failureReason = "WorkflowFailed"
		failureMessage = "Workflow for alert has failed"
	}

	alertConditionChanged := false
	if alertOpsFailed {
		alert.Status.Conditions = append(alert.Status.Conditions, *newCondition(alertv1alpha1.AlertFailedOpsWrofklow, v1.ConditionTrue, failureReason, failureMessage))
		alertConditionChanged = true
		now := metav1.Now()
		alert.Status.OpsStatus.CompletionTime = &now
		alert.Status.OpsStatus.Status = alertv1alpha1.OpsStatusFailed
		c.recorder.Event(&alert, v1.EventTypeWarning, failureReason, failureMessage)
	} else {
		if alertNeedSync && total == 0 {
			total, alert.Status.OpsStatus.TriggerStatus, createWorkflowErr = c.createWorkflowForAlert(ctx, &alert)
			if createWorkflowErr == nil {
				// alert.Status.Conditions = append(alert.Status.Conditions, *newCondition(alertv1alpha1.AlertSucceededCreateOpsWorkflow, v1.ConditionTrue, "", ""))
				alert.Status.OpsStatus.Status = alertv1alpha1.OpsStatusPending
				alert.Status.OpsStatus.Total = &total
				alertConditionChanged = true
				c.recorder.Event(&alert, v1.EventTypeNormal, "SucceededCreateOpsWorkflow", "Alert succeeded create ops workflow")
			}
		}

		if createWorkflowErr != nil {
			alert.Status.Conditions = append(alert.Status.Conditions, *newCondition(alertv1alpha1.AlertFailedCreateOpsWorkflow, v1.ConditionTrue, "", ""))
			alertConditionChanged = true
			c.recorder.Event(&alert, v1.EventTypeWarning, "FailedCreateOpsWorkflow", fmt.Sprintf("Alert failed create ops workflow: %v", createWorkflowErr))
		}

		complete := (total <= succeeded && createWorkflowErr == nil)
		if complete {
			alert.Status.Conditions = append(alert.Status.Conditions, *newCondition(alertv1alpha1.AlertCompleteOpsWrofklow, v1.ConditionTrue, "", ""))
			alertConditionChanged = true
			now := metav1.Now()
			alert.Status.OpsStatus.CompletionTime = &now
			alert.Status.OpsStatus.Status = alertv1alpha1.OpsStatusSucceeded
			c.recorder.Event(&alert, v1.EventTypeNormal, "Completed", "Alert Ops completed")
		}
	}

	forget = false

	if alert.Status.OpsStatus.Active != active || alert.Status.OpsStatus.Succeeded != succeeded || alert.Status.OpsStatus.Failed != failed || alertConditionChanged {
		alert.Status.OpsStatus.Active = active
		alert.Status.OpsStatus.Succeeded = succeeded
		alert.Status.OpsStatus.Failed = failed

		if err := c.updateStatusHandler(ctx, &alert); err != nil {
			return forget, err
		}
		forget = true
	}
	return forget, nil
}

func alertUntriggerWorkflow(alert *alertv1alpha1.AegisAlert) bool {
	return alert.Status.OpsStatus.Total == nil
}

func newCondition(conditionType alertv1alpha1.AlertOpsConditionType, status v1.ConditionStatus, reason, message string) *alertv1alpha1.AlertOpsCondition {
	return &alertv1alpha1.AlertOpsCondition{
		Type:               conditionType,
		Status:             status,
		LastProbeTime:      metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

func prepareWorkflowParameters(alert *alertv1alpha1.AegisAlert) map[string]interface{} {
	para := map[string]interface{}{
		"InvolvedObjectKind":      string(alert.Spec.InvolvedObject.Kind),
		"InvolvedObjectName":      alert.Spec.InvolvedObject.Name,
		"InvolvedObjectNamespace": alert.Spec.InvolvedObject.Namespace,
		"InvolvedObjectNode":      alert.Spec.InvolvedObject.Node,
	}

	if alert.Spec.Details != nil {
		for key, value := range alert.Spec.Details {
			para[key] = value
		}
	}

	for key, value := range alert.Annotations {
		para[key] = value
	}

	para["AlertName"] = alert.Name
	para["AlertNamespace"] = alert.Namespace
	return para
}

// createWorkflowForAlert is responsible for create workflows according to what is specified in the alert rule.
func (c *AlertController) createWorkflowForAlert(ctx context.Context, alert *alertv1alpha1.AegisAlert) (total int32, triggerStatus alertv1alpha1.AlertOpsTriggerStatusType, err error) {
	alertKey, err := nativecontroller.KeyFunc(alert)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for alert %v: %v", alert, err))
		return
	}

	if alertUntriggerWorkflow(alert) {
		// query rule engine to get template refs
		rule := &controller.MatchRule{
			Labels: alert.Labels,
			Condition: &controller.Condition{
				Type:   alert.Spec.Type,
				Status: string(alert.Spec.Status),
			},
		}
		var templateRefs []*v1.ObjectReference

		templateRefs, err = c.ruleEngineController.GetTemplateRefs(rule)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Get workflow template refs for alert %v: %v", alert, err))
			triggerStatus = alertv1alpha1.OpsTriggerStatusRuleError
			return
		}

		total = int32(len(templateRefs))
		if total == 0 {
			err = fmt.Errorf("No workflow template rule found")
			triggerStatus = alertv1alpha1.OpsTriggerStatusRuleNotFound
			go callback(c.lifecycleControl.OnNoOpsRule, alert, alertKey)
			return
		}

		if total > 1 {
			err = fmt.Errorf("More than one workflow template rule found")
			utilruntime.HandleError(fmt.Errorf("No workflow template(%v) found for alert: %v", templateRefs[0], alert))
			triggerStatus = alertv1alpha1.OpsTriggerStatusRuleTooManyFound
			return
		}

		var tpl string
		tpl, err = c.ruleEngineController.GetTemplateContentByRefs(templateRefs[0])
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("No workflow template(%v) found for alert: %v", templateRefs[0], alert))
			triggerStatus = alertv1alpha1.OpsTriggerStatusTemplateNotFound
			go callback(c.lifecycleControl.OnNoOpsTemplate, alert, alertKey)
			return
		}

		parameters := prepareWorkflowParameters(alert)
		var yamlContent string
		yamlContent, err = tools.RenderWorkflowTemplate(tpl, parameters)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Invalid workflow template for alert %v: %v", alert, err))
			triggerStatus = alertv1alpha1.OpsTriggerStatusTemplateInvalid
			go c.ruleEngineController.FailedExecuteTemplateCallback(templateRefs[0])
			go callback(c.lifecycleControl.OnFailedCreateOpsWorkflow, alert, alertKey)
			return
		}

		c.expectations.ExpectCreations(c.logger, alertKey, int(total))
		err = c.workflowControl.CreateWorkflowWithPlainContent(ctx, alert.Namespace, yamlContent, alert, metav1.NewControllerRef(alert, controllerKind))
		if err != nil {
			utilruntime.HandleError(err)
			klog.V(2).Infof("Failed creation, decrementing expectation for alert %s", alertKey)
			c.expectations.CreationObserved(c.logger, alertKey)
			triggerStatus = alertv1alpha1.OpsTriggerStatusTriggerFailed

			go c.ruleEngineController.FailedExecuteTemplateCallback(templateRefs[0])
			go callback(c.lifecycleControl.OnFailedCreateOpsWorkflow, alert, alertKey)
			return
		}

		go c.ruleEngineController.SucceedExecuteTemplateCallback(templateRefs[0])
		go callback(c.lifecycleControl.OnSucceedCreateOpsWorkflow, alert, alertKey)
		triggerStatus = alertv1alpha1.OpsTriggerStatusTriggered
	}
	return
}

// func removeTrackingFinalizerPatch(workflow *wfv1alpha1.Workflow) []byte {
// 	if !hasAlertTrackingFinalizer(workflow) {
// 		return nil
// 	}
// 	patch := map[string]interface{}{
// 		"metadata": map[string]interface{}{
// 			"$deleteFromPrimitiveList/finalizers": []string{alertv1alpha1.AlertTrackingFinalizer},
// 		},
// 	}
// 	patchBytes, _ := json.Marshal(patch)
// 	return patchBytes
// }

func (c *AlertController) updateAlertStatus(ctx context.Context, alert *alertv1alpha1.AegisAlert) error {
	newAlert, err := c.alertclientset.AegisV1alpha1().AegisAlerts(alert.Namespace).Get(ctx, alert.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	newAlert.Status = alert.Status
	_, err = c.alertclientset.AegisV1alpha1().AegisAlerts(alert.Namespace).Update(ctx, newAlert, metav1.UpdateOptions{})
	return err
}

func (c *AlertController) cleanAlert(ctx context.Context, alert *alertv1alpha1.AegisAlert) error {
	err := c.alertclientset.AegisV1alpha1().AegisAlerts(alert.Namespace).Delete(ctx, alert.Name, metav1.DeleteOptions{})
	return err
}

// func (c *AlertController) patchAlert(ctx context.Context, alert *alertv1alpha1.AegisAlert, data []byte) error {
// 	_, err := c.alertclientset.AegisV1alpha1().AegisAlerts(alert.Namespace).Patch(ctx, alert.Namespace, types.StrategicMergePatchType, data, metav1.PatchOptions{})
// 	return err
// }
