package nodecheck

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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

	nodecheckv1alpha1 "github.com/scitix/aegis/pkg/apis/nodecheck/v1alpha1"
	nodecheckclientset "github.com/scitix/aegis/pkg/generated/nodecheck/clientset/versioned"
	nodecheckInformer "github.com/scitix/aegis/pkg/generated/nodecheck/informers/externalversions/nodecheck/v1alpha1"
	nodecheckLister "github.com/scitix/aegis/pkg/generated/nodecheck/listers/nodecheck/v1alpha1"

	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"

	"github.com/scitix/aegis/pkg/controller"
	nativecontroller "k8s.io/kubernetes/pkg/controller"
)

func init() {
	nodecheckv1alpha1.AddToScheme(scheme.Scheme)
}

const (
	podDefaultUpdatePeriod = 500 * time.Millisecond
)

var controllerAgentName = "nodecheck-Controller"

var controllerKind = nodecheckv1alpha1.SchemeGroupVersion.WithKind("AegisNodeHealthCheck")

var callback func(callback func(nodecheck *nodecheckv1alpha1.AegisNodeHealthCheck) error, nodecheck *nodecheckv1alpha1.AegisNodeHealthCheck, key string) = func(callback func(nodecheck *nodecheckv1alpha1.AegisNodeHealthCheck) error, nodecheck *nodecheckv1alpha1.AegisNodeHealthCheck, key string) {
	name := runtime.FuncForPC(reflect.ValueOf(callback).Pointer()).Name()
	if err := callback(nodecheck); err != nil {
		klog.Errorf("fail to execute lifecycle callback %v for nodecheck %s: %v", name, key, err)
	} else {
		klog.V(9).Infof("succeed execute lifecycle callback %v callback for nodecheck %s", name, key)
	}
}

type NodeCheckController struct {
	kubeClient clientset.Interface

	nodecheckclientset nodecheckclientset.Interface

	lifecycleControl controller.AegisCallbackInterface

	// A TTLCache of pod create/delete each expect to use
	expectations nativecontroller.ControllerExpectationsInterface

	// lister
	lister nodecheckLister.AegisNodeHealthCheckLister

	podLister corelisters.PodLister

	cmLister corelisters.ConfigMapLister

	nodeLister corelisters.NodeLister

	podSynced cache.InformerSynced

	cmSynced cache.InformerSynced

	nodeSynced cache.InformerSynced

	synced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface

	// syncHandler func(ctx context.Context, podKey string) (bool, error)

	broadcaster record.EventBroadcaster
	recorder    record.EventRecorder

	podUpdatePeriod time.Duration

	enableFireNodeEvent bool

	logger klog.Logger
}

// add reacts to an NodeCheck creation
func (n *NodeCheckController) added(obj interface{}) {
	nodecheck := obj.(*nodecheckv1alpha1.AegisNodeHealthCheck)
	key, err := nativecontroller.KeyFunc(nodecheck)
	if err != nil {
		return
	}

	go callback(n.lifecycleControl.OnNodeCheckUpdate, nodecheck, key)

	n.enqueueDataLoad(obj)
}

// updated reacts to NodeCHeck update
func (n *NodeCheckController) updated(oldObj, newObj interface{}) {
	old := oldObj.(*nodecheckv1alpha1.AegisNodeHealthCheck)
	new := newObj.(*nodecheckv1alpha1.AegisNodeHealthCheck)
	if new.ResourceVersion == old.ResourceVersion {
		return
	}

	key, err := nativecontroller.KeyFunc(new)
	if err != nil {
		return
	}

	go callback(n.lifecycleControl.OnNodeCheckUpdate, new, key)

	n.enqueueDataLoad(newObj)
}

// deleted reacts to NodeCheck delet
func (n *NodeCheckController) deleted(obj interface{}) {
	dataload := obj.(*nodecheckv1alpha1.AegisNodeHealthCheck)
	_, err := nativecontroller.KeyFunc(dataload)
	if err != nil {
		return
	}

	n.enqueueDataLoad(obj)
}

func (n *NodeCheckController) enqueueDataLoad(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	n.workqueue.Add(key)
}

// N ewController returns a controller
func NewController(kubeclient kubernetes.Interface,
	nodecheckclient nodecheckclientset.Interface,
	nodecheckinformer nodecheckInformer.AegisNodeHealthCheckInformer,
	podinformer coreinformers.PodInformer,
	cminformer coreinformers.ConfigMapInformer,
	nodeinformer coreinformers.NodeInformer,
	lifecycleControl controller.AegisCallbackInterface,
	enableFireNodeEvent bool) *NodeCheckController {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: kubeclient.CoreV1().Events(v1.NamespaceAll)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: controllerAgentName})

	controller := &NodeCheckController{
		kubeClient:          kubeclient,
		nodecheckclientset:  nodecheckclient,
		lifecycleControl:    lifecycleControl,
		expectations:        nativecontroller.NewControllerExpectations(),
		lister:              nodecheckinformer.Lister(),
		podLister:           podinformer.Lister(),
		cmLister:            cminformer.Lister(),
		nodeLister:          nodeinformer.Lister(),
		workqueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "nodecheck"),
		broadcaster:         eventBroadcaster,
		recorder:            recorder,
		synced:              nodecheckinformer.Informer().HasSynced,
		podSynced:           podinformer.Informer().HasSynced,
		cmSynced:            cminformer.Informer().HasSynced,
		nodeSynced:          nodeinformer.Informer().HasSynced,
		podUpdatePeriod:     podDefaultUpdatePeriod,
		enableFireNodeEvent: enableFireNodeEvent,
		logger:              klog.NewKlogr(),
	}

	klog.Info("Setting up event handles")

	nodecheckinformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.added,
		UpdateFunc: controller.updated,
		DeleteFunc: controller.deleted,
	})

	podinformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addPod,
		UpdateFunc: controller.updatePod,
		DeleteFunc: controller.deletePod,
	})

	return controller
}

func (n *NodeCheckController) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer n.workqueue.ShutDown()

	klog.Info("Starting nodecheck controller")

	klog.Info("Waiting for nodecheck informer caches to sync")
	if ok := cache.WaitForNamedCacheSync("nodecheck", ctx.Done(), n.synced); !ok {
		return fmt.Errorf("failed to wait for nodecheck cache to sync")
	}

	klog.Info("Waiting for cm informer caches to sync")
	if ok := cache.WaitForNamedCacheSync("configmap", ctx.Done(), n.cmSynced); !ok {
		return fmt.Errorf("failed to wait for configmap cache to sync")
	}

	klog.Info("Waiting for pod informer caches to sync")
	if ok := cache.WaitForNamedCacheSync("pod", ctx.Done(), n.podSynced); !ok {
		return fmt.Errorf("failed to wait for pod cache to sync")
	}

	klog.Info("Waiting for node informer caches to sync")
	if ok := cache.WaitForNamedCacheSync("node", ctx.Done(), n.nodeSynced); !ok {
		return fmt.Errorf("failed to wait for node cache to sync")
	}

	klog.Info("Starting nodecheck workers")
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, n.runWorker, time.Second)
	}

	klog.Info("Started workers")

	<-ctx.Done()
	klog.Info("Shutting down workers.")
	return nil
}

func (n *NodeCheckController) runWorker(ctx context.Context) {
	for n.processNextWorkItem(ctx) {
	}

	klog.V(4).Infof("exit item process loop")
}

func (n *NodeCheckController) processNextWorkItem(ctx context.Context) bool {
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

	utilruntime.HandleError(fmt.Errorf("syncing nodecheck: %s", err))
	if !apierrors.IsConflict(err) {
		n.workqueue.AddRateLimited(obj)
	}

	return true
}

func (n *NodeCheckController) resolveControllerRef(namespace string, controllerRef *metav1.OwnerReference) *nodecheckv1alpha1.AegisNodeHealthCheck {
	if controllerRef.Kind != controllerKind.Kind {
		return nil
	}

	nodecheck, err := n.lister.AegisNodeHealthChecks(namespace).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if nodecheck.UID != controllerRef.UID {
		return nil
	}

	return nodecheck
}

func (n *NodeCheckController) addPod(obj interface{}) {
	pod := obj.(*v1.Pod)
	if pod.DeletionTimestamp != nil {
		n.deletePod(pod)
	}

	if controllerRef := metav1.GetControllerOf(pod); controllerRef != nil {
		nodecheck := n.resolveControllerRef(pod.Namespace, controllerRef)
		if nodecheck == nil {
			return
		}

		checkKey, err := nativecontroller.KeyFunc(nodecheck)
		if err != nil {
			return
		}

		n.expectations.CreationObserved(n.logger, checkKey)
		klog.V(4).Infof("enqueueing nodecheck %s for pod %s/%s added", checkKey, pod.Namespace, pod.Name)
		n.enqueueControllerPodUpdate(nodecheck, true)
		return
	}
}

func (n *NodeCheckController) updatePod(old, cur interface{}) {
	curPod := cur.(*v1.Pod)
	oldPod := old.(*v1.Pod)
	if curPod.ResourceVersion == oldPod.ResourceVersion {
		return
	}

	if curPod.DeletionTimestamp != nil {
		n.deletePod(curPod)
		return
	}

	immediate := (curPod.Status.Phase != v1.PodFailed) && (curPod.Status.Phase != v1.PodUnknown)

	curControllerRef := metav1.GetControllerOf(curPod)
	oldControllerRef := metav1.GetControllerOf(oldPod)
	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	if controllerRefChanged && oldControllerRef != nil {
		// The ControllerRef was changed. Sync the old controller
		if nodecheck := n.resolveControllerRef(oldPod.Namespace, oldControllerRef); nodecheck != nil {
			klog.V(4).Infof("enqueueing nodecheck %s/%s for update pod %s/%s controller ref", nodecheck.Namespace, nodecheck.Name, curPod.Namespace, curPod.Name)
			n.enqueueControllerPodUpdate(nodecheck, immediate)
		}
	}

	if curControllerRef != nil {
		dataload := n.resolveControllerRef(curPod.Namespace, curControllerRef)
		if dataload == nil {
			return
		}

		checkKey, err := nativecontroller.KeyFunc(dataload)
		if err != nil {
			return
		}

		klog.V(4).Infof("enqueueing nodecheck %s for pod %s/%s updated", checkKey, curPod.Namespace, curPod.Name)
		n.enqueueControllerPodUpdate(dataload, immediate)
		return
	}
}

func (n *NodeCheckController) deletePod(obj interface{}) {
	pod := obj.(*v1.Pod)

	controllerRef := metav1.GetControllerOf(pod)
	if controllerRef == nil {
		return
	}
	nodecheck := n.resolveControllerRef(pod.Namespace, controllerRef)
	if nodecheck == nil || IsNodeCheckFailed(nodecheck) {
		return
	}
	loadKey, err := nativecontroller.KeyFunc(nodecheck)
	if err != nil {
		return
	}
	klog.V(4).Infof("enqueueing nodecheck %s for pod %s/%s deleted", nodecheck, pod.Namespace, pod.Name)
	n.expectations.DeletionObserved(n.logger, loadKey)
}

func (n *NodeCheckController) enqueueController(obj interface{}, immediate bool) {
	n.enqueueControllerDelayed(obj, immediate, 0)
}

func (n *NodeCheckController) enqueueControllerPodUpdate(obj interface{}, immediate bool) {
	n.enqueueControllerDelayed(obj, immediate, n.podUpdatePeriod)
}

func (n *NodeCheckController) enqueueControllerDelayed(obj interface{}, immediate bool, delay time.Duration) {
	key, err := nativecontroller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %v: %v", obj, err))
	}

	n.workqueue.AddAfter(key, delay)
}

func (n *NodeCheckController) getPodsForNodeCheck(ctx context.Context, nodecheck *nodecheckv1alpha1.AegisNodeHealthCheck, withFinalizer bool) ([]*v1.Pod, error) {
	pods, err := n.podLister.Pods(nodecheck.Namespace).List(labels.Set(nodecheck.Labels).AsSelector())
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("List %d pods for nodecheck %s/%s", len(pods), nodecheck.Namespace, nodecheck.Name)

	podsToConnected := make([]*v1.Pod, 0)
	for _, pod := range pods {
		if controllerRef := metav1.GetControllerOf(pod); controllerRef != nil && controllerRef.Name == nodecheck.Name {
			podsToConnected = append(podsToConnected, pod)
		}
	}

	return podsToConnected, nil
}

func (n *NodeCheckController) syncHandler(ctx context.Context, key string) (forget bool, err error) {
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing nodecheck %q (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource: %s", key))
		return false, err
	}

	if len(namespace) == 0 || len(name) == 0 {
		return false, fmt.Errorf("invalid nodecheck key %q: either namespace or name is missing", key)
	}
	// get nodecheck resorce
	sharedNodeCheck, err := n.lister.AegisNodeHealthChecks(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("nodecheck has been deleted: %v", key)
			n.expectations.DeleteExpectations(n.logger, key)

			return true, nil
		}
		return false, err
	}

	nodecheck := *sharedNodeCheck.DeepCopy()

	// if nodecheck finished previously, we don't want to redo the termination
	if IsNodeCheckFinished(&nodecheck) {
		return true, nil
	}

	nodecheckNeedSync := n.expectations.SatisfiedExpectations(n.logger, key)

	pods, err := n.getPodsForNodeCheck(ctx, &nodecheck, true)
	var pod *v1.Pod
	// select first one
	if len(pods) > 0 {
		pod = pods[0]

		if len(pods) > 1 {
			klog.Warningf("got %d pods for nodecheck %v, expected 1", len(pods), key)
		}
	}

	if nodecheck.Status.StartTime == nil {
		now := metav1.Now()
		nodecheck.Status.StartTime = &now
	}

	desiredPhase := nodecheckv1alpha1.CheckStatusPending
	if pod != nil {
		switch pod.Status.Phase {
		case v1.PodPending:
			desiredPhase = nodecheckv1alpha1.CheckStatusPending
		case v1.PodRunning:
			desiredPhase = nodecheckv1alpha1.CheckStatusRunning
		case v1.PodSucceeded:
			desiredPhase = nodecheckv1alpha1.CheckStatusSucceeded
		case v1.PodFailed:
			desiredPhase = nodecheckv1alpha1.CheckStatusFailed
		case v1.PodUnknown:
			desiredPhase = nodecheckv1alpha1.CheckStatusUnknown
		}
	}

	var createPodErr error
	nodecheckPodFailed := false
	var failureReason string
	var failureMessage string

	if pod != nil && pod.Status.Phase == v1.PodFailed {
		nodecheckPodFailed = true
		failureReason = "NodeCheckFailed"
		failureMessage = "pod for nodecheck has failed"
	}

	nodecheckConditionChanged := false
	if nodecheckPodFailed {
		nodecheck.Status.Conditions = append(nodecheck.Status.Conditions, *newCondition(nodecheckv1alpha1.CheckFailed, v1.ConditionTrue, failureReason, failureMessage))
		nodecheckConditionChanged = true
		now := metav1.Now()
		desiredPhase = nodecheckv1alpha1.CheckStatusFailed
		nodecheck.Status.CompletionTime = &now
		n.recorder.Event(&nodecheck, v1.EventTypeWarning, failureReason, failureMessage)
	} else if pod != nil && pod.Status.Phase == v1.PodSucceeded {
		nodecheckConditionChanged = true
		now := metav1.Now()
		nodecheck.Status.CompletionTime = &now
		nodecheck.Status.Conditions = append(nodecheck.Status.Conditions, *newCondition(nodecheckv1alpha1.CheckSucceed, v1.ConditionTrue, "", ""))
		n.recorder.Event(&nodecheck, v1.EventTypeNormal, "Completed", "NodeCheck pod completed")

		// to fetch result
		results, err := n.fetchReesult(ctx, &nodecheck)
		if err != nil {
			nodecheck.Status.Conditions = append(nodecheck.Status.Conditions, *newCondition(nodecheckv1alpha1.CheckFailedFetchResult, v1.ConditionTrue, "FetchResultFailed", err.Error()))
			desiredPhase = nodecheckv1alpha1.CheckStatusFailed
		} else {
			nodecheck.Status.Results = results

			// try sync conditions to node
			if n.enableFireNodeEvent {
				go n.tryFireNodeEvent(ctx, &nodecheck)
			}
		}
	} else {
		if nodecheckNeedSync && pod == nil {
			createPodErr = n.createPodForNodeCheck(ctx, &nodecheck)

			if createPodErr == nil {
				nodecheck.Status.Conditions = append(nodecheck.Status.Conditions, *newCondition(nodecheckv1alpha1.CheckSucceededCreatePod, v1.ConditionTrue, "", ""))
				nodecheck.Status.Status = nodecheckv1alpha1.CheckStatusPending
				nodecheckConditionChanged = true
				n.recorder.Event(&nodecheck, v1.EventTypeNormal, "SucceededCreateNodeCheckPod", "NodeCheck succeeded create pod")
			}
		}

		if createPodErr != nil && !apierrors.IsAlreadyExists(createPodErr) {
			nodecheck.Status.Conditions = append(nodecheck.Status.Conditions, *newCondition(nodecheckv1alpha1.CheckFailedCreatePod, v1.ConditionTrue, "", ""))
			nodecheckConditionChanged = true
			now := metav1.Now()
			nodecheck.Status.CompletionTime = &now
			desiredPhase = nodecheckv1alpha1.CheckStatusFailed
			n.recorder.Event(&nodecheck, v1.EventTypeWarning, "FailedCreateNodeCheckPod", fmt.Sprintf("NodeCheck failed create pod: %v", createPodErr))
		}
	}

	if nodecheck.Status.Status != desiredPhase || nodecheckConditionChanged {
		nodecheck.Status.Status = desiredPhase

		if err := n.updateStatus(ctx, &nodecheck); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (n *NodeCheckController) fetchReesult(ctx context.Context, nodecheck *nodecheckv1alpha1.AegisNodeHealthCheck) (nodecheckv1alpha1.ResultInfos, error) {
	pods, err := n.getPodsForNodeCheck(ctx, nodecheck, true)
	if err != nil || len(pods) != 1 {
		err := fmt.Errorf("failed to list pods for nodecheck %v: %v", nodecheck, err)
		return nil, err
	}

	pod := pods[0]
	req := n.kubeClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}

	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(buf.Bytes()), "\n")
	var yamlPart string
	for i, line := range lines {
		if strings.HasPrefix(line, "results:") {
			yamlPart = strings.Join(lines[i:], "\n")
		}
	}

	result := nodecheckv1alpha1.Results{
		Results: make(map[string]nodecheckv1alpha1.ResourceInfos),
	}

	err = yaml.Unmarshal([]byte(yamlPart), &result)
	if err != nil {
		return nil, err
	}

	return result.Results, nil
}

func (n *NodeCheckController) tryFireNodeEvent(ctx context.Context, nodecheck *nodecheckv1alpha1.AegisNodeHealthCheck) (err error) {
	results := nodecheck.Status.Results
	node, err := n.nodeLister.Get(nodecheck.Spec.Node)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get node %s: %v", nodecheck.Spec.Node, err))
		return err
	}

	for _, infos := range results {
		for _, info := range infos {
			if info.Status {
				n.recorder.Event(node, v1.EventTypeWarning, info.Condition, info.Message)
			}
		}
	}

	return nil
}

func newCondition(conditionType nodecheckv1alpha1.CheckConditionType, status v1.ConditionStatus, reason, message string) *nodecheckv1alpha1.CheckCondition {
	return &nodecheckv1alpha1.CheckCondition{
		Type:               conditionType,
		Status:             status,
		LastProbeTime:      metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

func (n *NodeCheckController) createPodForNodeCheck(ctx context.Context, nodecheck *nodecheckv1alpha1.AegisNodeHealthCheck) (err error) {
	checkKey, err := nativecontroller.KeyFunc(nodecheck)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for nodecheck %v: %v", nodecheck, err))
		return
	}

	node := nodecheck.Spec.Node
	_, err = n.nodeLister.Get(node)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get node %s: %v", node, err))
		return
	}

	selector, err := metav1.LabelSelectorAsSelector(&nodecheck.Spec.RuleConfigmapSelector)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get selector: %v", err))
		return
	}

	configmaps, err := n.kubeClient.CoreV1().ConfigMaps(nodecheck.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})

	volumes := make([]v1.Volume, 0)
	volumeMounts := make([]v1.VolumeMount, 0)

	for _, configmap := range configmaps.Items {
		name := configmap.Name
		volumeName := fmt.Sprintf("volume-%s", name)
		mountPath := fmt.Sprintf("/nodecheck/config/%s", name)
		volumes = append(volumes, v1.Volume{
			Name: volumeName,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: name,
					},
				},
			},
		})

		volumeMounts = append(volumeMounts, v1.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
		})
	}

	spec := nodecheck.Spec.Template.Spec
	spec.RestartPolicy = v1.RestartPolicyNever
	spec.NodeName = node

	if len(spec.Containers) != 1 {
		return fmt.Errorf("expected 1 container, got %d", len(spec.Containers))
	}
	spec.Containers[0].VolumeMounts = append(spec.Containers[0].VolumeMounts, volumeMounts...)
	spec.Volumes = append(spec.Volumes, volumes...)

	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", nodecheck.Name),
			Labels:       nodecheck.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(nodecheck, controllerKind),
			},
		},
		Spec: spec,
	}

	n.expectations.ExpectCreations(n.logger, checkKey, 1)
	_, err = n.kubeClient.CoreV1().Pods(nodecheck.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	return err
}

func (n *NodeCheckController) updateStatus(ctx context.Context, nodecheck *nodecheckv1alpha1.AegisNodeHealthCheck) error {
	newNodeCheck, err := n.nodecheckclientset.AegisV1alpha1().AegisNodeHealthChecks(nodecheck.Namespace).Get(ctx, nodecheck.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	newNodeCheck.Status = nodecheckv1alpha1.AegisNodeHealthCheckStatus{
		Conditions:     nodecheck.Status.Conditions,
		Status:         nodecheck.Status.Status,
		StartTime:      nodecheck.Status.StartTime,
		CompletionTime: nodecheck.Status.CompletionTime,
		Results:        nodecheck.Status.Results,
	}

	_, err = n.nodecheckclientset.AegisV1alpha1().AegisNodeHealthChecks(nodecheck.Namespace).Update(ctx, newNodeCheck, metav1.UpdateOptions{})
	return err
}
