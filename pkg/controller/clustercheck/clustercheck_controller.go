package clustercheck

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/robfig/cron/v3"
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

	clustercheckv1alpha1 "github.com/scitix/aegis/pkg/apis/clustercheck/v1alpha1"
	clustercheckclientset "github.com/scitix/aegis/pkg/generated/clustercheck/clientset/versioned"
	clustercheckInformer "github.com/scitix/aegis/pkg/generated/clustercheck/informers/externalversions/clustercheck/v1alpha1"
	clustercheckLister "github.com/scitix/aegis/pkg/generated/clustercheck/listers/clustercheck/v1alpha1"

	nodecheckv1alpha1 "github.com/scitix/aegis/pkg/apis/nodecheck/v1alpha1"
	nodecheckclientset "github.com/scitix/aegis/pkg/generated/nodecheck/clientset/versioned"
	nodecheckInformer "github.com/scitix/aegis/pkg/generated/nodecheck/informers/externalversions/nodecheck/v1alpha1"
	nodecheckLister "github.com/scitix/aegis/pkg/generated/nodecheck/listers/nodecheck/v1alpha1"

	"github.com/scitix/aegis/pkg/controller"
	nativecontroller "k8s.io/kubernetes/pkg/controller"
)

func init() {
	clustercheckv1alpha1.AddToScheme(scheme.Scheme)
}

const (
	defaultUpdatePeriod = 500 * time.Millisecond
)

var (
	controllerAgentName = "clustercheck-Controller"

	controllerKind = clustercheckv1alpha1.SchemeGroupVersion.WithKind("AegisClusterHealthCheck")

	nextScheduleDelta = 100 * time.Millisecond
)

type ClusterCheckController struct {
	kubeClient clientset.Interface

	clustercheckclientset clustercheckclientset.Interface

	nodecheckclientset nodecheckclientset.Interface

	// A TTLCache of pod create/delete each expect to use
	expectations nativecontroller.ControllerExpectationsInterface

	// lister
	lister clustercheckLister.AegisClusterHealthCheckLister
	synced cache.InformerSynced

	nodeCheckLister nodecheckLister.AegisNodeHealthCheckLister
	nodeCheckSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface

	// syncHandler func(ctx context.Context, podKey string) (bool, error)

	broadcaster record.EventBroadcaster
	recorder    record.EventRecorder

	nodeCheckUpdatePeriod time.Duration

	logger klog.Logger
}

// add reacts to an ClusterCheck creation
func (n *ClusterCheckController) added(obj interface{}) {
	check := obj.(*clustercheckv1alpha1.AegisClusterHealthCheck)
	_, err := nativecontroller.KeyFunc(check)
	if err != nil {
		return
	}

	n.enqueueClusterCheck(obj)
}

// updated reacts to NodeCHeck update
func (n *ClusterCheckController) updated(oldObj, newObj interface{}) {
	old := oldObj.(*clustercheckv1alpha1.AegisClusterHealthCheck)
	new := newObj.(*clustercheckv1alpha1.AegisClusterHealthCheck)
	if new.ResourceVersion == old.ResourceVersion {
		return
	}

	_, err := nativecontroller.KeyFunc(new)
	if err != nil {
		return
	}

	n.enqueueClusterCheck(newObj)
}

// deleted reacts to ClusterCheck delet
func (n *ClusterCheckController) deleted(obj interface{}) {
	check := obj.(*clustercheckv1alpha1.AegisClusterHealthCheck)
	_, err := nativecontroller.KeyFunc(check)
	if err != nil {
		return
	}

	n.enqueueClusterCheck(obj)
}

func (n *ClusterCheckController) enqueueClusterCheck(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	n.workqueue.Add(key)
}

// NewController returns a controller
func NewController(kubeclient kubernetes.Interface,
	clustercheckclient clustercheckclientset.Interface,
	clustercheckinformer clustercheckInformer.AegisClusterHealthCheckInformer,
	nodecheckclient nodecheckclientset.Interface,
	nodecheckinformer nodecheckInformer.AegisNodeHealthCheckInformer) *ClusterCheckController {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: kubeclient.CoreV1().Events(v1.NamespaceAll)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: controllerAgentName})

	controller := &ClusterCheckController{
		kubeClient:            kubeclient,
		clustercheckclientset: clustercheckclient,
		expectations:          nativecontroller.NewControllerExpectations(),
		lister:                clustercheckinformer.Lister(),
		nodecheckclientset:    nodecheckclient,
		nodeCheckLister:       nodecheckinformer.Lister(),
		workqueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "clustercheck"),
		broadcaster:           eventBroadcaster,
		recorder:              recorder,
		synced:                nodecheckinformer.Informer().HasSynced,
		nodeCheckSynced:       nodecheckinformer.Informer().HasSynced,
		nodeCheckUpdatePeriod: defaultUpdatePeriod,
		logger:                klog.NewKlogr(),
	}

	klog.Info("Setting up event handles")

	clustercheckinformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.added,
		UpdateFunc: controller.updated,
		DeleteFunc: controller.deleted,
	})

	nodecheckinformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addNodeCheck,
		UpdateFunc: controller.updateNodeCheck,
		DeleteFunc: controller.deleteNodeCheck,
	})

	return controller
}

func (n *ClusterCheckController) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer n.workqueue.ShutDown()

	klog.Info("Starting clustercheck controller")

	klog.Info("Waiting for clustercheck informer caches to sync")
	if ok := cache.WaitForNamedCacheSync("clustercheck", ctx.Done(), n.synced); !ok {
		return fmt.Errorf("failed to wait for clustercheck cache to sync")
	}

	klog.Info("Waiting for nodecheck informer caches to sync")
	if ok := cache.WaitForNamedCacheSync("nodecheck", ctx.Done(), n.nodeCheckSynced); !ok {
		return fmt.Errorf("failed to wait for nodecheck cache to sync")
	}

	klog.Info("Starting clustercheck workers")
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, n.runWorker, time.Second)
	}

	klog.Info("Started workers")

	<-ctx.Done()
	klog.Info("Shutting down workers.")
	return nil
}

func (n *ClusterCheckController) runWorker(ctx context.Context) {
	for n.processNextWorkItem(ctx) {
	}

	klog.V(4).Infof("exit item process loop")
}

func (n *ClusterCheckController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := n.workqueue.Get()

	klog.V(6).Infof("received object %s", obj)

	if shutdown {
		klog.V(4).Infof("workqueue shutdown, exit process.")
		return false
	}
	defer n.workqueue.Done(obj)

	forget, err := n.syncHandler(ctx, obj.(string))
	if err == nil {
		if forget {
			n.workqueue.Forget(obj)
		}
		return true
	}

	klog.Errorf("error sync object %s: %s", obj, err)

	utilruntime.HandleError(fmt.Errorf("syncing clustercheck: %s", err))
	if !apierrors.IsConflict(err) {
		n.workqueue.AddRateLimited(obj)
	}

	return true
}

func (n *ClusterCheckController) resolveControllerRef(namespace string, controllerRef *metav1.OwnerReference) *clustercheckv1alpha1.AegisClusterHealthCheck {
	if controllerRef.Kind != controllerKind.Kind {
		return nil
	}

	check, err := n.lister.AegisClusterHealthChecks(namespace).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if check.UID != controllerRef.UID {
		return nil
	}

	return check
}

func (n *ClusterCheckController) addNodeCheck(obj interface{}) {
	nodecheck := obj.(*nodecheckv1alpha1.AegisNodeHealthCheck)
	if nodecheck.DeletionTimestamp != nil {
		n.deleteNodeCheck(nodecheck)
	}

	if controllerRef := metav1.GetControllerOf(nodecheck); controllerRef != nil {
		check := n.resolveControllerRef(nodecheck.Namespace, controllerRef)
		if nodecheck == nil {
			return
		}

		checkKey, err := nativecontroller.KeyFunc(check)
		if err != nil {
			return
		}

		n.expectations.CreationObserved(n.logger, checkKey)
		klog.V(4).Infof("enqueueing clustercheck %s for nodecheck %s/%s added", checkKey, nodecheck.Namespace, nodecheck.Name)
		n.enqueueControllerNodeCheckUpdate(check, true)
		return
	}
}

func (n *ClusterCheckController) updateNodeCheck(old, cur interface{}) {
	curNodeCheck := cur.(*nodecheckv1alpha1.AegisNodeHealthCheck)
	oldNodeCheck := old.(*nodecheckv1alpha1.AegisNodeHealthCheck)
	if curNodeCheck.ResourceVersion == oldNodeCheck.ResourceVersion {
		return
	}

	if curNodeCheck.DeletionTimestamp != nil {
		n.deleteNodeCheck(curNodeCheck)
		return
	}

	immediate := true // (curNodeCheck.Status.Status != nodecheckv1alpha1.CheckStatusFailed) && (curNodeCheck.Status.Status != nodecheckv1alpha1.CheckStatusUnknown)

	curControllerRef := metav1.GetControllerOf(curNodeCheck)
	oldControllerRef := metav1.GetControllerOf(oldNodeCheck)
	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	if controllerRefChanged && oldControllerRef != nil {
		// The ControllerRef was changed. Sync the old controller
		if check := n.resolveControllerRef(oldNodeCheck.Namespace, oldControllerRef); check != nil {
			klog.V(4).Infof("enqueueing clustercheck %s/%s for update nodecheck %s/%s controller ref", check.Namespace, check.Name, curNodeCheck.Namespace, curNodeCheck.Name)
			n.enqueueControllerNodeCheckUpdate(check, immediate)
		}
	}

	if curControllerRef != nil {
		check := n.resolveControllerRef(curNodeCheck.Namespace, curControllerRef)
		if check == nil {
			return
		}

		checkKey, err := nativecontroller.KeyFunc(check)
		if err != nil {
			return
		}

		klog.V(4).Infof("enqueueing clustercheck %s for nodecheck %s/%s updated", checkKey, curNodeCheck.Namespace, curNodeCheck.Name)
		n.enqueueControllerNodeCheckUpdate(check, immediate)
		return
	}
}

func (n *ClusterCheckController) deleteNodeCheck(obj interface{}) {
	nodecheck := obj.(*nodecheckv1alpha1.AegisNodeHealthCheck)

	controllerRef := metav1.GetControllerOf(nodecheck)
	if controllerRef == nil {
		return
	}
	check := n.resolveControllerRef(nodecheck.Namespace, controllerRef)
	if check == nil || IsClusterCheckFinsihed(check) {
		return
	}
	loadKey, err := nativecontroller.KeyFunc(nodecheck)
	if err != nil {
		return
	}
	klog.V(4).Infof("enqueueing clustercheck %s for nodecheck %s/%s deleted", check, nodecheck.Namespace, nodecheck.Name)
	n.expectations.DeletionObserved(n.logger, loadKey)
}

// func (n *ClusterCheckController) enqueueController(obj interface{}, immediate bool) {
// 	n.enqueueControllerDelayed(obj, immediate, 0)
// }

func (n *ClusterCheckController) enqueueControllerNodeCheckUpdate(obj interface{}, immediate bool) {
	n.enqueueControllerDelayed(obj, immediate, n.nodeCheckUpdatePeriod)
}

func (n *ClusterCheckController) enqueueControllerDelayed(obj interface{}, immediate bool, delay time.Duration) {
	key, err := nativecontroller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %v: %v", obj, err))
	}

	if immediate {
		n.workqueue.Add(key)
		return
	}

	klog.V(6).Infof("enqueueing clustercheck %s for delay %s, count: %d", key, delay, n.workqueue.Len())
	n.workqueue.AddAfter(key, delay)
}

func (n *ClusterCheckController) getNodeChecksForNodeCheck(ctx context.Context, check *clustercheckv1alpha1.AegisClusterHealthCheck, withFinalizer bool) ([]*nodecheckv1alpha1.AegisNodeHealthCheck, error) {
	nodechecks, err := n.nodeCheckLister.AegisNodeHealthChecks(check.Namespace).List(labels.Set(check.Labels).AsSelector())
	if err != nil {
		return nil, err
	}

	nodeChecksToConnected := make([]*nodecheckv1alpha1.AegisNodeHealthCheck, 0)
	for _, nodecheck := range nodechecks {
		if controllerRef := metav1.GetControllerOf(nodecheck); controllerRef != nil && controllerRef.Name == check.Name {
			nodeChecksToConnected = append(nodeChecksToConnected, nodecheck)
		}
	}

	klog.V(4).Infof("List %d nodecheck for check %s/%s", len(nodeChecksToConnected), check.Namespace, check.Name)

	return nodeChecksToConnected, nil
}

func (n *ClusterCheckController) syncHandler(ctx context.Context, key string) (forget bool, err error) {
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing clustercheck %q (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource: %s", key))
		return false, err
	}

	if len(namespace) == 0 || len(name) == 0 {
		return false, fmt.Errorf("invalid clustercheck key %q: either namespace or name is missing", key)
	}

	// get nodecheck resorce
	sharedCheck, err := n.lister.AegisClusterHealthChecks(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("clustercheck has been deleted: %v", key)
			n.expectations.DeleteExpectations(n.logger, key)

			return true, nil
		}
		return false, err
	}

	check := *sharedCheck.DeepCopy()

	// if clustercheck finished previously, we don't want to redo the termination
	if IsClusterCheckFinsihed(&check) {
		// cleanup nodechecks
		n.cleanupNodeChecksForClusterCheck(ctx, &check)

		// next sched
		sched, err := cron.ParseStandard(check.Spec.Schedule)
		if err != nil {
			klog.V(2).Infof("failed to parse schedule %q: %v", check.Spec.Schedule, err)
			n.recorder.Eventf(&check, v1.EventTypeWarning, "UnparseableSchedule", "failed to parse schedule %q: %v", check.Spec.Schedule, err)
			return true, nil
		}

		scheduledTime, err := getNextScheduleTime(&check, time.Now(), sched, n.recorder)
		if err != nil {
			klog.V(2).Infof("Invalid schedule %q: %v", check.Spec.Schedule, err)
			n.recorder.Eventf(&check, v1.EventTypeWarning, "InvalidSchedule", "Invalid schedule %q: %v", check.Spec.Schedule, err)
			return true, nil
		}

		if scheduledTime == nil {
			klog.V(2).Infof("No unmet start times for check %q", check.Name)

			t := nextScheduledTimeDuration(sched, time.Now())

			n.enqueueControllerDelayed(&check, false, *t)
			return true, nil
		}

		inWindow := scheduledTime.Add(time.Second * -1).Before(time.Now())
		if inWindow {
			check.Status = clustercheckv1alpha1.AegisClusterHealthCheckStatus{}
			n.updateStatus(ctx, &check)
			return true, nil
		}

		t := nextScheduledTimeDuration(sched, time.Now())
		n.enqueueControllerDelayed(&check, false, *t)
		return true, nil
	}

	checkNeedSync := n.expectations.SatisfiedExpectations(n.logger, key)

	nodechecks, err := n.getNodeChecksForNodeCheck(ctx, &check, true)
	if err != nil {
		return false, nil
	}

	activeNodeCheck, succeededNodeCheck, failedNodeCheck := controller.FilterActiveNodeCheck(nodechecks), controller.FilterSucceededNodeCheck(nodechecks), controller.FilterFailedNodeCheck(nodechecks)
	active, succeeded, failed := int32(len(activeNodeCheck)), int32(len(succeededNodeCheck)), int32(len(failedNodeCheck))
	desired := int32(0)
	if check.Status.Desired != nil {
		desired = *check.Status.Desired
	}

	if check.Status.StartTime == nil {
		now := metav1.Now()
		check.Status.StartTime = &now
	}

	desiredPhase := n.analysisPhase(ctx, &check, nodechecks)
	klog.V(2).Infof("check %s/%s phase: %s, active: %d, succeeded: %d, failed: %d, desired: %d", check.Namespace, check.Name, desiredPhase, active, succeeded, failed, desired)

	var createErr error
	var created int32 = 0
	conditionChanged := false
	if desiredPhase == clustercheckv1alpha1.CheckPhaseCompleted {
		// sync results
		if check.Status.CheckResults == nil {
			check.Status.CheckResults, _ = n.mergeResult(ctx, nodechecks)
			check.Status.Phase = clustercheckv1alpha1.CheckPhaseCompleted

			check.Status.Conditions = append(check.Status.Conditions, *newCondition(clustercheckv1alpha1.CheckCompleted, v1.ConditionTrue, "", ""))
			now := metav1.Now()
			check.Status.CompletionTime = &now
			conditionChanged = true

			go func() {
				time.Sleep(time.Second * 10)
				n.cleanupNodeChecksForClusterCheck(ctx, &check)
			}()
		}
	} else if desiredPhase == clustercheckv1alpha1.CheckPhaseChecking {
		if check.Status.Phase != clustercheckv1alpha1.CheckPhaseChecking {
			check.Status.Phase = desiredPhase
			conditionChanged = true

			// send a timeout obj
			// diff = 10
			klog.V(2).Infof("cluster check %s/%s is in checking phase, enqueue for timeout %ds delay", check.Namespace, check.Name, *check.Spec.Timeout)

			n.enqueueControllerDelayed(&check, false, time.Second*time.Duration(*check.Spec.Timeout+10))
		}
	} else {
		if checkNeedSync && desired == 0 {
			desired, created, createErr = n.createNodeChecksForClusterCheck(ctx, &check)
			if createErr == nil {
				check.Status.Conditions = append(check.Status.Conditions, *newCondition(clustercheckv1alpha1.CheckSucceededCreateNodeCheck, v1.ConditionTrue, "", ""))
				check.Status.Phase = clustercheckv1alpha1.CheckPhasePending
				check.Status.Desired = &desired
				conditionChanged = true
				n.recorder.Event(&check, v1.EventTypeNormal, "SucceededCreatedNodeChecks", fmt.Sprintf("cluster check succeeded created %d node checks", created))
			}
		}

		if createErr != nil {
			check.Status.Conditions = append(check.Status.Conditions, *newCondition(clustercheckv1alpha1.CheckFailedCreateNodeCheck, v1.ConditionTrue, "", createErr.Error()))
			check.Status.Phase = clustercheckv1alpha1.CheckPhaseFailed
			conditionChanged = true
			n.recorder.Event(&check, v1.EventTypeNormal, "FailedCreatedNodeChecks", fmt.Sprintf("cluster check failed created node checks: %s", createErr))
		}
	}

	forget = false

	if check.Status.Active != active || check.Status.Failed != failed || check.Status.Succeeded != succeeded || conditionChanged {

		check.Status.Active = active
		check.Status.Failed = failed
		check.Status.Succeeded = succeeded

		if err := n.updateStatus(ctx, &check); err != nil {
			return false, err
		}
		forget = true
	}

	return forget, nil
}

func nextScheduledTimeDuration(sched cron.Schedule, now time.Time) *time.Duration {
	t := sched.Next(now).Add(nextScheduleDelta).Sub(now)
	return &t
}

func (n *ClusterCheckController) analysisPhase(ctx context.Context, check *clustercheckv1alpha1.AegisClusterHealthCheck, nodechecks []*nodecheckv1alpha1.AegisNodeHealthCheck) clustercheckv1alpha1.CheckPhase {
	// pending
	if check.Status.Desired == nil {
		return clustercheckv1alpha1.CheckPhasePending
	}

	// by timeout
	if IsClusterCheckTimeout(check) {
		return clustercheckv1alpha1.CheckPhaseCompleted
	}

	// by nodecheck
	phase := clustercheckv1alpha1.CheckPhaseCompleted
	for _, check := range nodechecks {
		switch check.Status.Status {
		case nodecheckv1alpha1.CheckStatusRunning:
			fallthrough
		case nodecheckv1alpha1.CheckStatusPending:
			fallthrough
		case nodecheckv1alpha1.CheckStatusUnknown:
			return clustercheckv1alpha1.CheckPhaseChecking
		case nodecheckv1alpha1.CheckStatusFailed:
			fallthrough
		case nodecheckv1alpha1.CheckStatusSucceeded:
			continue
		default:
			return clustercheckv1alpha1.CheckPhasePending
		}
	}
	return phase
}

func (n *ClusterCheckController) cleanupNodeChecksForClusterCheck(ctx context.Context, clustercheck *clustercheckv1alpha1.AegisClusterHealthCheck) error {
	nodechecks, err := n.getNodeChecksForNodeCheck(ctx, clustercheck, true)
	if err != nil {
		return err
	}

	for _, nodecheck := range nodechecks {
		if err := n.nodecheckclientset.AegisV1alpha1().AegisNodeHealthChecks(nodecheck.Namespace).Delete(ctx, nodecheck.Name, metav1.DeleteOptions{}); err != nil {
			n.recorder.Event(clustercheck, v1.EventTypeWarning, fmt.Sprintf("Failed to cleanup nodecheck %s/%s", nodecheck.Namespace, nodecheck.Name), err.Error())
			utilruntime.HandleError(fmt.Errorf("Failed to cleanup nodecheck %s/%s: %v", nodecheck.Namespace, nodecheck.Name, err))
			continue
		}
	}

	return nil
}

func (n *ClusterCheckController) mergeResult(ctx context.Context, nodechecks []*nodecheckv1alpha1.AegisNodeHealthCheck) (*clustercheckv1alpha1.CheckResults, error) {
	results := &clustercheckv1alpha1.CheckResults{
		CheckType: clustercheckv1alpha1.CheckTypeNode,
		Results:   make([]clustercheckv1alpha1.CheckResult, 0),
	}

	for _, nodecheck := range nodechecks {
		cr := clustercheckv1alpha1.CheckResult{
			Name:        nodecheck.Spec.Node,
			ResultInfos: make(nodecheckv1alpha1.ResultInfos),
		}

		for module, resultinfos := range nodecheck.Status.Results {
			ts := make([]nodecheckv1alpha1.ResourceInfo, 0)
			for _, info := range resultinfos {
				if info.Status {
					ts = append(ts, info)
				}
			}

			if len(ts) > 0 {
				cr.ResultInfos[module] = ts
			}
		}

		if len(cr.ResultInfos) > 0 {
			results.Results = append(results.Results, cr)
		}
	}

	return results, nil
}

func newCondition(conditionType clustercheckv1alpha1.CheckConditionType, status v1.ConditionStatus, reason, message string) *clustercheckv1alpha1.CheckCondition {
	return &clustercheckv1alpha1.CheckCondition{
		Type:               conditionType,
		Status:             status,
		LastProbeTime:      metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

func (n *ClusterCheckController) createNodeChecksForClusterCheck(ctx context.Context, check *clustercheckv1alpha1.AegisClusterHealthCheck) (total int32, created int32, err error) {
	checkKey, err := nativecontroller.KeyFunc(check)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for clustercheck %v: %v", check, err))
		return
	}

	// selector nodes
	nodeSelector := check.Spec.Template.Spec.NodeSelector
	nodes, err := n.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(nodeSelector).AsSelector().String(),
	})
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't list nodes: %v", err))
		return
	}

	filterdNodes := make([]string, 0)
	// filter by tolerance
	tolerations := check.Spec.Template.Spec.Tolerations
	for _, node := range nodes.Items {
		tolerate := len(node.Spec.Taints) == 0
		for _, toleration := range tolerations {
			if !ToleratesTaint(toleration, node.Spec.Taints) {
				tolerate = false
				break
			} else {
				tolerate = true
			}
		}

		if tolerate {
			filterdNodes = append(filterdNodes, node.Name)
		}
	}

	total = int32(len(filterdNodes))

	existsingNodeChecks, err := n.getNodeChecksForNodeCheck(ctx, check, true)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't list nodechecks: %v", err))
		return
	}

	existsMap := make(map[string]bool)
	for _, nodecheck := range existsingNodeChecks {
		existsMap[nodecheck.Spec.Node] = true
	}

	n.expectations.ExpectCreations(n.logger, checkKey, len(filterdNodes)-len(existsingNodeChecks))
	for _, node := range filterdNodes {
		if existsMap[node] {
			continue
		}

		nodecheck := &nodecheckv1alpha1.AegisNodeHealthCheck{
			TypeMeta: metav1.TypeMeta{
				Kind:       "AegisNodeHealthCheck",
				APIVersion: "v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: fmt.Sprintf("%s-", check.Name),
				Namespace:    check.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(check, controllerKind),
				},
				Labels: check.Labels,
			},
			Spec: nodecheckv1alpha1.AegisNodeHealthCheckSpec{
				Node:                  node,
				RuleConfigmapSelector: check.Spec.RuleConfigmapSelector,
				Template:              check.Spec.Template,
			},
		}

		_, err = n.nodecheckclientset.AegisV1alpha1().AegisNodeHealthChecks(nodecheck.Namespace).Create(ctx, nodecheck, metav1.CreateOptions{})
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to create AegisNodeHealthCheck %s: %v", check.Name, err))
			return
		}
		created++

		// time.Sleep(time.Second)
	}

	return
}

func (n *ClusterCheckController) updateStatus(ctx context.Context, check *clustercheckv1alpha1.AegisClusterHealthCheck) error {
	newCheck, err := n.clustercheckclientset.AegisV1alpha1().AegisClusterHealthChecks(check.Namespace).Get(ctx, check.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	newCheck.Status = clustercheckv1alpha1.AegisClusterHealthCheckStatus{
		Conditions:     check.Status.Conditions,
		Phase:          check.Status.Phase,
		StartTime:      check.Status.StartTime,
		CompletionTime: check.Status.CompletionTime,
		CheckResults:   check.Status.CheckResults,
		Active:         check.Status.Active,
		Desired:        check.Status.Desired,
		Failed:         check.Status.Failed,
		Succeeded:      check.Status.Succeeded,
	}

	_, err = n.clustercheckclientset.AegisV1alpha1().AegisClusterHealthChecks(check.Namespace).Update(ctx, newCheck, metav1.UpdateOptions{})
	return err
}
