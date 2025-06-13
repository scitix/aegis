package rule

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	rulev1alpha1 "github.com/scitix/aegis/pkg/apis/rule/v1alpha1"
	clientset "github.com/scitix/aegis/pkg/generated/rule/clientset/versioned"
	informers "github.com/scitix/aegis/pkg/generated/rule/informers/externalversions/rule/v1alpha1"
	listers "github.com/scitix/aegis/pkg/generated/rule/listers/rule/v1alpha1"

	templateclientset "github.com/scitix/aegis/pkg/generated/template/clientset/versioned"
	templateInformers "github.com/scitix/aegis/pkg/generated/template/informers/externalversions/template/v1alpha1"
	templateListers "github.com/scitix/aegis/pkg/generated/template/listers/template/v1alpha1"
)

func init() {
	rulev1alpha1.AddToScheme(scheme.Scheme)
}

const controllerAgentName = "AlertRuleController"

const (
	SuccessSynced         = "Synced"
	MessageResourceSynced = "Rule synced successfully"

	RuleRecorded = "Recorded"
)

type RuleController struct {
	mu         sync.Mutex
	kubeClient kubernetes.Interface

	// alert rule clientset
	ruleclinetset     clientset.Interface
	templateclientset templateclientset.Interface

	lister         listers.AegisAlertOpsRuleLister
	templateLister templateListers.AegisOpsTemplateLister

	ruleCache map[string]*rulev1alpha1.AegisAlertOpsRule

	synced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface

	recorder record.EventRecorder
}

func (c *RuleController) addOrUpdateFromCache(key string, rule *rulev1alpha1.AegisAlertOpsRule) {
	klog.V(4).Infof("Add or Update rule: %s", key)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ruleCache[key] = rule.DeepCopy()
}

func (c *RuleController) deleteFromCache(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.ruleCache, key)
}

// add reacts to an AegisAlertOpsRule creation
func (c *RuleController) added(obj interface{}) {
	c.enqueueRule(obj)
}

// updated reacts to AegisAlertOpsRule update
func (c *RuleController) updated(oldObj, newObj interface{}) {
	oldRule := oldObj.(*rulev1alpha1.AegisAlertOpsRule)
	newRule := newObj.(*rulev1alpha1.AegisAlertOpsRule)
	if newRule.ResourceVersion == oldRule.ResourceVersion {
		return
	}

	c.enqueueRule(newObj)
}

// deleted reacts to AegisAlertOpsRule delet
func (c *RuleController) deleted(obj interface{}) {
	c.enqueueRule(obj)
}

func (c *RuleController) enqueueRule(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.workqueue.Add(key)
}

// NewController is the controller implement for aegis alert ops rule
func NewController(
	kubeclientset kubernetes.Interface,
	ruleclientset clientset.Interface,
	templateclientset templateclientset.Interface,
	ruleInformer informers.AegisAlertOpsRuleInformer,
	templateInformer templateInformers.AegisOpsTemplateInformer) *RuleController {

	// Create event broadcaster
	klog.V(4).Info("Creating aegis rule event boradcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events(v1.NamespaceAll)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: controllerAgentName})

	controller := &RuleController{
		kubeClient:        kubeclientset,
		ruleclinetset:     ruleclientset,
		templateclientset: templateclientset,
		lister:            ruleInformer.Lister(),
		templateLister:    templateInformer.Lister(),
		ruleCache:         make(map[string]*rulev1alpha1.AegisAlertOpsRule),
		synced:            ruleInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerAgentName),
		recorder:          recorder,
	}

	klog.Info("Setting up event handles")

	// setting up event handle for rule resource
	ruleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.added,
		UpdateFunc: controller.updated,
		DeleteFunc: controller.deleted,
	})

	return controller
}

func (c *RuleController) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Info("Starting rule controller")

	klog.Info("Waiting for rule informer caches to sync")
	if ok := cache.WaitForNamedCacheSync("rule", ctx.Done(), c.synced); !ok {
		return fmt.Errorf("failed to wait for cache to sync")
	}

	klog.Info("Starting rule workers")
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	klog.Info("Started workers")

	<-ctx.Done()
	klog.Info("Shutting down workers.")
	return nil
}

func (c *RuleController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *RuleController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)

		key, ok := obj.(string)
		if !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %v", obj))
			return nil
		}

		if err := c.syncHandler(key); err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing %s: %s requeuing", key, err.Error())
		}

		c.workqueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *RuleController) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource: %s", key))
		return err
	}

	// get rule resorce
	rule, err := c.lister.AegisAlertOpsRules(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("rule '%s' in work queue no longer exists", key))
			c.deleteFromCache(key)
			return nil
		}
		return err
	}

	err = c.updateStatus(context.Background(), rule)
	if err != nil {
		return err
	}
	c.addOrUpdateFromCache(key, rule)

	c.recorder.Event(rule, v1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *RuleController) updateStatus(ctx context.Context, rule *rulev1alpha1.AegisAlertOpsRule) error {
	newRule, err := c.ruleclinetset.AegisV1alpha1().AegisAlertOpsRules(rule.Namespace).Get(ctx, rule.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	newRule.Status.Status = RuleRecorded

	_, err = c.ruleclinetset.AegisV1alpha1().AegisAlertOpsRules(rule.Namespace).Update(context.TODO(), newRule, metav1.UpdateOptions{})
	return err
}
