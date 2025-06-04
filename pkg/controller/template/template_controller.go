package template

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

	templatev1alpha1 "gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/template/v1alpha1"
	clientset "gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/template/clientset/versioned"
	informers "gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/template/informers/externalversions/template/v1alpha1"
	listers "gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/template/listers/template/v1alpha1"
)

func init() {
	templatev1alpha1.AddToScheme(scheme.Scheme)
}

const controllerAgentName = "TemplateController"

const (
	SuccessSynced         = "Synced"
	MessageResourceSynced = "Template synced successfully"

	StatusRecorded = "Recorded"
)

type TemplateController struct {
	mu sync.Mutex

	kubeClient kubernetes.Interface

	// alert rule clientset
	templateclinetset clientset.Interface

	lister listers.AegisOpsTemplateLister
	synced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface

	recorder record.EventRecorder
}

// add reacts to an AegisOpsTemplate creation
func (c *TemplateController) added(obj interface{}) {
	c.enqueueTemplate(obj)
}

// updated reacts to AegisOpsTemplate update
func (c *TemplateController) updated(oldObj, newObj interface{}) {
	oldTemplate := oldObj.(*templatev1alpha1.AegisOpsTemplate)
	newTemplate := newObj.(*templatev1alpha1.AegisOpsTemplate)
	if newTemplate.ResourceVersion == oldTemplate.ResourceVersion {
		return
	}

	c.enqueueTemplate(newObj)
}

// deleted reacts to AegisOpsTemplate delet
func (c *TemplateController) deleted(obj interface{}) {
	c.enqueueTemplate(obj)
}

func (c *TemplateController) enqueueTemplate(obj interface{}) {
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
	templateclientset clientset.Interface,
	templateInformer informers.AegisOpsTemplateInformer) *TemplateController {

	// Create event broadcaster
	klog.V(4).Info("Creating aegis template event boradcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events(v1.NamespaceAll)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: controllerAgentName})

	controller := &TemplateController{
		kubeClient:        kubeclientset,
		templateclinetset: templateclientset,
		lister:            templateInformer.Lister(),
		synced:            templateInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerAgentName),
		recorder:          recorder,
	}

	klog.Info("Setting up event handles")

	// setting up event handle for template resource
	templateInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.added,
		UpdateFunc: controller.updated,
		DeleteFunc: controller.deleted,
	})

	return controller
}

func (c *TemplateController) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Info("Starting template controller")

	klog.Info("Waiting for template informer caches to sync")
	if ok := cache.WaitForNamedCacheSync("template", ctx.Done(), c.synced); !ok {
		return fmt.Errorf("failed to wait for cache to sync")
	}

	klog.Info("Starting template workers")
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	klog.Info("Started workers")

	<-ctx.Done()
	klog.Info("Shutting down workers.")
	return nil
}

func (c *TemplateController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *TemplateController) processNextWorkItem(ctx context.Context) bool {
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

func (c *TemplateController) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource: %s", key))
		return err
	}

	// get template resorce
	rule, err := c.lister.AegisOpsTemplates(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("template '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	err = c.updateStatus(context.Background(), rule)
	if err != nil {
		return err
	}

	c.recorder.Event(rule, v1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *TemplateController) updateStatus(ctx context.Context, template *templatev1alpha1.AegisOpsTemplate) error {
	newTemplate, err := c.templateclinetset.AegisV1alpha1().AegisOpsTemplates(template.Namespace).Get(ctx, template.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	newTemplate.Status.Status = StatusRecorded
	_, err = c.templateclinetset.AegisV1alpha1().AegisOpsTemplates(template.Namespace).Update(context.TODO(), newTemplate, metav1.UpdateOptions{})
	return err
}
