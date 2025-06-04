package diagnosis

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	"gitlab.scitix-inner.ai/k8s/aegis/pkg/controller"

	diagnosisv1alpha1 "gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/diagnosis/v1alpha1"
	diagnosisclientset "gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/diagnosis/clientset/versioned"
	diagnosisInformer "gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/diagnosis/informers/externalversions/diagnosis/v1alpha1"
	diagnosisLister "gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/diagnosis/listers/diagnosis/v1alpha1"
)

func init() {
	diagnosisv1alpha1.AddToScheme(scheme.Scheme)
}

const (
	SucceededDiagnosis        = "Succeeded"
	FailedDiagnosis           = "Failed"
	MessageSucceededDiagnosis = "Diagnosis successfully"
	MessageFailededDiagnosis  = "Diagnosis Failed"
)

var controllerAgentName = "diagnosis-Controller"

var controllerKind = diagnosisv1alpha1.SchemeGroupVersion.WithKind("AegisDiagnosis")

type DiagnosisController struct {
	kubeClient clientset.Interface

	diagnosis *Diagnosis

	diagnosisclientset diagnosisclientset.Interface

	// a store for diagnosis
	lister diagnosisLister.AegisDiagnosisLister

	// diagnosis that need to be updated
	workqueue workqueue.RateLimitingInterface

	broadcaster record.EventBroadcaster
	recorder    record.EventRecorder

	diagnosisSynced cache.InformerSynced

	// timeout for a diagnosis
	timeout time.Duration

	logger klog.Logger
}

// add reacts to an AegisDiagnosis creation
func (c *DiagnosisController) added(obj interface{}) {
	c.enqueueDiagnosis(obj)
}

// updated reacts to AegisDiagnosis update
func (c *DiagnosisController) updated(oldObj, newObj interface{}) {
	oldDiagnosis := oldObj.(*diagnosisv1alpha1.AegisDiagnosis)
	newDiagnosis := newObj.(*diagnosisv1alpha1.AegisDiagnosis)
	if newDiagnosis.ResourceVersion == oldDiagnosis.ResourceVersion {
		return
	}

	c.enqueueDiagnosis(newObj)
}

// deleted reacts to AegisDiagnosis delet
func (c *DiagnosisController) deleted(obj interface{}) {
	c.enqueueDiagnosis(obj)
}

func (c *DiagnosisController) enqueueDiagnosis(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.workqueue.Add(key)
}

// NewController create a new aegis diagnosis controller
// kubeclient
// diagnoseclient: diagnose resource controller
// diagnoseinformer: diagnose informer
// timeout: diagosis timeout
func NewController(kubeclient kubernetes.Interface,
	diagnosiscleint diagnosisclientset.Interface,
	diagnosisinformer diagnosisInformer.AegisDiagnosisInformer,
	timeout time.Duration,
	backend string,
	language string,
	explain bool,
	noCache bool,
) (*DiagnosisController, error) {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: kubeclient.CoreV1().Events(v1.NamespaceAll)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: controllerAgentName})

	controller := &DiagnosisController{
		kubeClient:         kubeclient,
		diagnosisclientset: diagnosiscleint,
		lister:             diagnosisinformer.Lister(),
		workqueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "diagnosis"),
		broadcaster:        eventBroadcaster,
		recorder:           recorder,
		diagnosisSynced:    diagnosisinformer.Informer().HasSynced,
		timeout:            timeout,
		logger:             klog.NewKlogr(),
	}

	dignosis, err := NewDiagnosis(backend, language, noCache, explain, nil)
	if err != nil {
		return nil, err
	}
	controller.diagnosis = dignosis

	klog.Info("Setting up event handles")

	diagnosisinformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.added,
		UpdateFunc: controller.updated,
		DeleteFunc: controller.deleted,
	})

	return controller, nil
}

func (c *DiagnosisController) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Info("Starting diagnosis controller")

	klog.Info("Waiting for diagnosis informer caches to sync")
	if ok := cache.WaitForNamedCacheSync("diagnosis", ctx.Done(), c.diagnosisSynced); !ok {
		return fmt.Errorf("failed to wait for cache to sync")
	}

	klog.Info("Starting diagnosis workers")
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, 2*time.Second)
	}

	klog.Info("Started workers")

	<-ctx.Done()
	klog.Info("Shutting down workers.")
	return nil
}

func (c *DiagnosisController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *DiagnosisController) processNextWorkItem(ctx context.Context) bool {
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

func (c *DiagnosisController) enqueueDiagnosisDelayed(obj interface{}, delay time.Duration) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %v: %s", obj, err))
	}

	c.workqueue.AddAfter(key, delay)
}

func (c *DiagnosisController) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource: %s", key))
		return err
	}

	// get diagnosis resorce
	sharedDiagnosis, err := c.lister.AegisDiagnosises(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("diagnosis '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	diagnosis := sharedDiagnosis.DeepCopy()
	// diagnosis has finished
	if IsDiagnoseFinished(diagnosis) {
		expired, ttl := CheckDiagnoseExpireTTL(diagnosis)
		if expired && ttl > 0 {
			klog.V(4).Infof("Diagnose %v ttl second: %d", key, ttl)
			c.enqueueDiagnosisDelayed(key, time.Duration(ttl)*time.Second)
		} else if expired {
			// cleanup diagnose
			klog.V(4).Infof("Diagnose %v ttl expired", key)
			if err := c.cleanDiagnosis(context.Background(), diagnosis); err != nil {
				klog.V(4).ErrorS(err, "Failed to delete diagnose %v: %v", key, err)
				return err
			}
		}
		return nil
	}

	object := diagnosis.Spec.Object
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	begin := metav1.Now()
	diagnosis.Status.StartTime = &begin

	result, explain, err := c.diagnosis.RunDiagnosis(ctx, string(object.Kind), object.Namespace, object.Name)

	end := metav1.Now()
	diagnosis.Status.CompletionTime = &end
	if err != nil {
		c.recorder.Event(diagnosis, v1.EventTypeWarning, FailedDiagnosis, MessageFailededDiagnosis)
		errMsg := fmt.Sprintf("Dignosis fialed: %s", err)
		diagnosis.Status.ErrorResult = &errMsg
		diagnosis.Status.Phase = diagnosisv1alpha1.DiagnosisPhaseFailed
	} else {
		c.recorder.Event(diagnosis, v1.EventTypeNormal, SucceededDiagnosis, MessageSucceededDiagnosis)
		diagnosis.Status.Result = result
		diagnosis.Status.Explain = &explain
		diagnosis.Status.Phase = diagnosisv1alpha1.DiagnosisPhaseCompleted
	}

	return c.updateStatus(context.Background(), diagnosis)
}

func (c *DiagnosisController) updateStatus(ctx context.Context, diagnosis *diagnosisv1alpha1.AegisDiagnosis) error {
	newDiagnosis, err := c.diagnosisclientset.AegisV1alpha1().AegisDiagnosises(diagnosis.Namespace).Get(ctx, diagnosis.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	newDiagnosis.Status = diagnosisv1alpha1.AegisDiagnosisStatus{
		Result:         diagnosis.Status.Result,
		Explain:        diagnosis.Status.Explain,
		ErrorResult:    diagnosis.Status.ErrorResult,
		Phase:          diagnosis.Status.Phase,
		StartTime:      diagnosis.Status.StartTime,
		CompletionTime: diagnosis.Status.CompletionTime,
	}

	_, err = c.diagnosisclientset.AegisV1alpha1().AegisDiagnosises(diagnosis.Namespace).Update(ctx, newDiagnosis, metav1.UpdateOptions{})
	return err
}

func (c *DiagnosisController) cleanDiagnosis(ctx context.Context, diagnosis *diagnosisv1alpha1.AegisDiagnosis) error {
	err := c.diagnosisclientset.AegisV1alpha1().AegisDiagnosises(diagnosis.Namespace).Delete(ctx, diagnosis.Name, metav1.DeleteOptions{})
	return err
}
