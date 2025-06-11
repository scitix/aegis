package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/scitix/aegis/api/models"
	"github.com/scitix/aegis/internal/k8s"
	"github.com/scitix/aegis/pkg/apis/alert/v1alpha1"
	"github.com/scitix/aegis/pkg/controller"
	"github.com/scitix/aegis/pkg/controller/alert"
	"github.com/scitix/aegis/pkg/controller/clustercheck"
	"github.com/scitix/aegis/pkg/controller/diagnosis"
	"github.com/scitix/aegis/pkg/controller/nodecheck"
	"github.com/scitix/aegis/pkg/controller/rule"
	"github.com/scitix/aegis/pkg/controller/template"
	"github.com/scitix/aegis/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"

	alertv1alpha1 "github.com/scitix/aegis/pkg/apis/alert/v1alpha1"
	alertclientset "github.com/scitix/aegis/pkg/generated/alert/clientset/versioned"
	alertInformers "github.com/scitix/aegis/pkg/generated/alert/informers/externalversions"

	ruleclientset "github.com/scitix/aegis/pkg/generated/rule/clientset/versioned"
	ruleInformers "github.com/scitix/aegis/pkg/generated/rule/informers/externalversions"

	templateclientset "github.com/scitix/aegis/pkg/generated/template/clientset/versioned"
	templateinformers "github.com/scitix/aegis/pkg/generated/template/informers/externalversions"

	diagnosisclientset "github.com/scitix/aegis/pkg/generated/diagnosis/clientset/versioned"
	diagnosisInformers "github.com/scitix/aegis/pkg/generated/diagnosis/informers/externalversions"

	nodecheckclientset "github.com/scitix/aegis/pkg/generated/nodecheck/clientset/versioned"
	nodecheckInformers "github.com/scitix/aegis/pkg/generated/nodecheck/informers/externalversions"

	clustercheckclientset "github.com/scitix/aegis/pkg/generated/clustercheck/clientset/versioned"
	clustercheckInformers "github.com/scitix/aegis/pkg/generated/clustercheck/informers/externalversions"

	wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	wfInformers "github.com/argoproj/argo-workflows/v3/pkg/client/informers/externalversions"
	"github.com/google/uuid"
)

type Configuration struct {
	Config               *rest.Config
	Client               clientset.Interface
	PublishNamespace     string
	SystemParas          map[string]string
	ResyncPeriod         time.Duration // force sycn period for watch
	SyncWorkers          int           // watch for workqueue
	EnableLeaderElection bool          // enable leader election
	ElectionID           string        // for leader election

	EnableAlert               bool
	DefaultTTLAfterOpsSucceed int32
	DefaultTTLAfterOpsFailed  int32
	DefaultTTLAfterNoOps      int32

	// diagnosis
	EnableDiagnosis        bool
	DiagnosisLanguage      string
	DiagnosisEnableExplain bool
	DiagnosisEnableCache   bool

	// ai
	AiBackend string

	EnableHealthcheck bool
	// nodecheck fire event
	EnableFireNodeEvent bool
}

type AegisController struct {
	cfg *Configuration

	// alert interface
	alertInterface controller.AlertControllerInterface

	// informer
	alertInformer    alertInformers.SharedInformerFactory
	workflowInformer wfInformers.SharedInformerFactory
	templateInformer templateinformers.SharedInformerFactory
	ruleInformer     ruleInformers.SharedInformerFactory
	diagnosisInfomer diagnosisInformers.SharedInformerFactory
	// checkInfomer     checkInformers.SharedInformerFactory
	nodecheckInformer    nodecheckInformers.SharedInformerFactory
	clustercheckInformer clustercheckInformers.SharedInformerFactory

	sharedInformer informers.SharedInformerFactory

	// manager alert
	alertController *alert.AlertController

	// manager rule
	ruleController *rule.RuleController

	// manager workflow template
	templateController *template.TemplateController

	// manager diagnosis
	diagnosisController *diagnosis.DiagnosisController

	// manager check
	// checkController *check.CheckController
	nodecheckController *nodecheck.NodeCheckController

	clustercheckController *clustercheck.ClusterCheckController
}

func NewAegisController(cfg *Configuration) (*AegisController, error) {
	ruleclientInterface, err := ruleclientset.NewForConfig(cfg.Config)
	if err != nil {
		return nil, fmt.Errorf("fail to create rule client interface: %v", err)
	}
	ruleInformer := ruleInformers.NewSharedInformerFactory(ruleclientInterface, cfg.ResyncPeriod)

	templateclientset, err := templateclientset.NewForConfig(cfg.Config)
	if err != nil {
		return nil, fmt.Errorf("fail to create template client interface: %v", err)
	}
	templateInformer := templateinformers.NewSharedInformerFactory(templateclientset, cfg.ResyncPeriod)

	alertclientInterface, err := alertclientset.NewForConfig(cfg.Config)
	if err != nil {
		return nil, fmt.Errorf("fail to create aegis alert controller: %v", err)
	}
	alertInformer := alertInformers.NewSharedInformerFactory(alertclientInterface, cfg.ResyncPeriod)
	aInformer := alertInformer.Aegis().V1alpha1().AegisAlerts()

	workflowclientset, err := wfclientset.NewForConfig(cfg.Config)
	if err != nil {
		return nil, fmt.Errorf("fail to create workflow client interface: %v", err)
	}
	workflowInformer := wfInformers.NewSharedInformerFactory(workflowclientset, cfg.ResyncPeriod)

	diagnosisclientset, err := diagnosisclientset.NewForConfig(cfg.Config)
	if err != nil {
		return nil, fmt.Errorf("fail to create diagnosis client interface: %v", err)
	}
	diagnosisInformer := diagnosisInformers.NewSharedInformerFactory(diagnosisclientset, cfg.ResyncPeriod)

	// checkclientset, err := checkclientset.NewForConfig(cfg.Config)
	// if err != nil {
	// 	return nil, fmt.Errorf("fail to create check client interface: %v", err)
	// }
	// checkinformer := checkInformers.NewSharedInformerFactory(checkclientset, cfg.ResyncPeriod)

	nodecheckclientset, err := nodecheckclientset.NewForConfig(cfg.Config)
	if err != nil {
		return nil, fmt.Errorf("fail to create nodecheck client interface: %v", err)
	}
	nodecheckInformer := nodecheckInformers.NewSharedInformerFactory(nodecheckclientset, cfg.ResyncPeriod)

	clustercheckclientset, err := clustercheckclientset.NewForConfig(cfg.Config)
	if err != nil {
		return nil, fmt.Errorf("fail to create clustercheck client interface: %v", err)
	}
	clustercheckInformer := clustercheckInformers.NewSharedInformerFactory(clustercheckclientset, cfg.ResyncPeriod)

	sharedInformers := informers.NewSharedInformerFactory(cfg.Client, cfg.ResyncPeriod)
	podInformer := sharedInformers.Core().V1().Pods()
	cmInformer := sharedInformers.Core().V1().ConfigMaps()
	nodeInformer := sharedInformers.Core().V1().Nodes()

	// alert callback interface
	lifecycle := newLifeCycle()
	metricsController := metrics.NewMetricsController()

	lifecycle.register("metrics", metricsController)
	// lifecycle.register("notify", notifyContoller)

	// create template controller
	templateController := template.NewController(cfg.Client, templateclientset, templateInformer.Aegis().V1alpha1().AegisOpsTemplates())
	ruleController := rule.NewController(cfg.Client, ruleclientInterface, templateclientset, ruleInformer.Aegis().V1alpha1().AegisAlertOpsRules(), templateInformer.Aegis().V1alpha1().AegisOpsTemplates())
	diagnosisController, err := diagnosis.NewController(cfg.Client, diagnosisclientset, diagnosisInformer.Aegis().V1alpha1().AegisDiagnosises(), 60*time.Second, cfg.AiBackend, cfg.DiagnosisLanguage, cfg.DiagnosisEnableExplain, !cfg.DiagnosisEnableCache)
	if err != nil {
		return nil, fmt.Errorf("fail to create diagnosis controller: %v", err)
	}

	alertController := alert.NewController(cfg.Client, alertclientInterface, workflowclientset, ruleController, workflowInformer.Argoproj().V1alpha1().Workflows(), aInformer, lifecycle)
	// checkController := check.NewController(cfg.Client, checkclientset, workflowclientset, workflowInformer.Argoproj().V1alpha1().Workflows(), checkinformer.Aegis().V1alpha1().AegisChecks())
	nodecheckController := nodecheck.NewController(cfg.Client, nodecheckclientset, nodecheckInformer.Aegis().V1alpha1().AegisNodeHealthChecks(), podInformer, cmInformer, nodeInformer, lifecycle, cfg.EnableFireNodeEvent)
	clustercheckController := clustercheck.NewController(cfg.Client, clustercheckclientset, clustercheckInformer.Aegis().V1alpha1().AegisClusterHealthChecks(), nodecheckclientset, nodecheckInformer.Aegis().V1alpha1().AegisNodeHealthChecks())

	n := &AegisController{
		cfg: cfg,
		alertInterface: &controller.RealAlertController{
			AlertClient: alertclientInterface,
			AlertLister: aInformer.Lister(),
		},
		sharedInformer:   sharedInformers,
		alertInformer:    alertInformer,
		workflowInformer: workflowInformer,
		ruleInformer:     ruleInformer,
		templateInformer: templateInformer,
		diagnosisInfomer: diagnosisInformer,
		// checkInfomer:       checkinformer,
		nodecheckInformer:    nodecheckInformer,
		clustercheckInformer: clustercheckInformer,
		alertController:      alertController,
		ruleController:       ruleController,
		templateController:   templateController,
		diagnosisController:  diagnosisController,
		// checkController:    checkController,
		nodecheckController:    nodecheckController,
		clustercheckController: clustercheckController,
	}
	return n, nil
}

func (c *AegisController) Run(ctx context.Context) error {
	var err error
	if c.cfg.EnableLeaderElection {
		lock := &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      c.cfg.ElectionID,
				Namespace: k8s.AegisPodDetail.Namespace,
			},
			Client: c.cfg.Client.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: k8s.AegisPodDetail.Name,
			},
		}

		ttl := 30 * time.Second
		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   ttl,
			RenewDeadline:   ttl / 2,
			RetryPeriod:     ttl / 4,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(ctx context.Context) {
					klog.InfoS("I am a new leader")
					err = c.run(ctx)
				},
				OnStoppedLeading: func() {
					klog.InfoS("I am not leader anymore")
				},
				OnNewLeader: func(identity string) {
					klog.InfoS("New leader elected", "identity", identity)
				},
			},
			Name: "Aegis Controller Lease",
		})
	} else {
		err = c.run(ctx)
	}

	if err != nil {
		return err
	}

	return nil
}

func (c *AegisController) run(ctx context.Context) error {
	workers := int(c.cfg.SyncWorkers)

	c.sharedInformer.Start(ctx.Done())

	var wg sync.WaitGroup
	wg.Add(6)

	errChan := make(chan error, 5)

	go func() {
		defer wg.Done()

		if !c.cfg.EnableAlert {
			return
		}

		c.alertInformer.Start(ctx.Done())
		c.workflowInformer.Start(ctx.Done())
		if err := c.alertController.Run(ctx, workers); err != nil {
			errChan <- fmt.Errorf("error running alert controller: %s", err.Error())
		}
	}()

	go func() {
		defer wg.Done()

		if !c.cfg.EnableAlert {
			return
		}

		c.ruleInformer.Start(ctx.Done())
		if err := c.ruleController.Run(ctx, workers); err != nil {
			errChan <- fmt.Errorf("error running rule controller: %s", err.Error())
		}
	}()

	go func() {
		defer wg.Done()

		if !c.cfg.EnableAlert {
			return
		}

		c.templateInformer.Start(ctx.Done())
		if err := c.templateController.Run(ctx, workers); err != nil {
			errChan <- fmt.Errorf("error running template controller: %s", err.Error())
		}
	}()

	go func() {
		defer wg.Done()

		if !c.cfg.EnableDiagnosis {
			return
		}

		c.diagnosisInfomer.Start(ctx.Done())
		if err := c.diagnosisController.Run(ctx, workers); err != nil {
			errChan <- fmt.Errorf("error running template controller: %s", err.Error())
		}
	}()

	go func() {
		defer wg.Done()

		if !c.cfg.EnableHealthcheck {
			return
		}

		c.nodecheckInformer.Start(ctx.Done())
		if err := c.nodecheckController.Run(ctx, workers); err != nil {
			errChan <- fmt.Errorf("error running nodecheck controller: %s", err.Error())
		}
	}()

	go func() {
		defer wg.Done()

		if !c.cfg.EnableHealthcheck {
			return
		}

		c.clustercheckInformer.Start(ctx.Done())
		if err := c.clustercheckController.Run(ctx, workers); err != nil {
			errChan <- fmt.Errorf("error running clustercheck controller: %s", err.Error())
		}
	}()

	wg.Wait()

	// close chan
	close(errChan)

	var errstrings []string
	for err := range errChan {
		if err != nil {
			errstrings = append(errstrings, err.Error())
		}
	}

	if len(errstrings) > 0 {
		return fmt.Errorf(strings.Join(errstrings, "\n"))
	}

	return nil
}

func getGeneratename(_alert *models.Alert) string {
	generateName := strings.ToLower(fmt.Sprintf("%s-%s-", _alert.AlertSourceType, _alert.Type))

	m := regexp.MustCompile(`[\W_]`)
	generateName = m.ReplaceAllString(generateName, "-")
	return generateName
}

func (c *AegisController) getNodeNameIfExists(ctx context.Context, _alert *models.Alert) (string, error) {
	switch alertv1alpha1.AlertObjectKind(_alert.InvolvedObject.Kind) {
	case alertv1alpha1.NodeKind:
		return _alert.InvolvedObject.Name, nil
	case alertv1alpha1.PodKind:
		if len(_alert.InvolvedObject.Node) > 0 {
			return _alert.InvolvedObject.Node, nil
		}
		if pod, err := c.cfg.Client.CoreV1().Pods(_alert.InvolvedObject.Namespace).Get(ctx, _alert.InvolvedObject.Name, metav1.GetOptions{}); err != nil {
			return "", err
		} else {
			return pod.Spec.NodeName, nil
		}
	default:
		return "", nil
	}
}

var filterKeys map[string]bool = map[string]bool{
	"alertname": true,
	//"node":        true,
	"pod":         true,
	"namespace":   true,
	"instance":    true,
	"cluster":     true,
	"description": true,
	"container":   true,
	"endpoint":    true,
	"env":         true,
	"job":         true,
	"prometheus":  true,
	"service":     true,
}

const (
	ValidLabelValueFormat = "^[A-Za-z0-9][-A-Za-z0-9_.]*[A-Za-z0-9]$"
)

func (c *AegisController) filterLabels(labels map[string]string) map[string]string {
	filters := make(map[string]string)
	for key, value := range labels {
		if match, _ := regexp.MatchString(ValidLabelValueFormat, value); match && !filterKeys[key] {
			filters[key] = value
		}
	}
	return filters
}

func (c *AegisController) CreateOrUpdateAlert(ctx context.Context, _alert *models.Alert) error {
	if _alert == nil {
		return fmt.Errorf("empty alert entity")
	}

	if _alert.Status == models.AlertStatusResolved {
		return c.tryPatchAlertStatus(ctx, _alert)
	} else {
		// update running alert count
		todos, err := c.tryFetchInCompletedAlerts(ctx, _alert)
		if err != nil {
			return err
		}

		if len(todos) > 0 {
			klog.V(2).Infof("found %d alert(s) for fingerprint: %s, skipping alert creation", len(todos), _alert.FingerPrint)
			return c.incurAlertCount(ctx, todos[0])
		}
	}

	node, err := c.getNodeNameIfExists(ctx, _alert)
	if err != nil {
		return err
	}

	labels := c.filterLabels(_alert.Details)
	labels["alert-source-type"] = string(_alert.AlertSourceType)
	labels["alert-type"] = string(_alert.Type)
	labels["alert-status"] = string(_alert.Status)
	labels["fingerprint"] = _alert.FingerPrint

	// unique alert resource
	labels["uuid"] = uuid.New().String()

	// severity
	severity := labels["severity"]
	alert := &alertv1alpha1.AegisAlert{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      labels,
			Annotations: c.cfg.SystemParas,
		},
		Spec: alertv1alpha1.AegisAlertSpec{
			TTLStrategy: &v1alpha1.TTLStrategy{
				SecondsAfterSuccess: &c.cfg.DefaultTTLAfterOpsSucceed,
				SecondsAfterFailure: &c.cfg.DefaultTTLAfterOpsFailed,
				SecondsAfterNoOps:   &c.cfg.DefaultTTLAfterNoOps,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Source:   string(_alert.AlertSourceType),
			Type:     string(_alert.Type),
			Status:   v1alpha1.AlertStatusType(_alert.Status),
			Severity: severity,
			InvolvedObject: alertv1alpha1.AegisAlertObject{
				Kind:      alertv1alpha1.AlertObjectKind(_alert.InvolvedObject.Kind),
				Name:      _alert.InvolvedObject.Name,
				Namespace: _alert.InvolvedObject.Namespace,
				Node:      node,
			},
			Details: _alert.Details,
		},
		Status: alertv1alpha1.AegisAlertStatus{
			Status: _alert.Status,
			Count:  1,
		},
	}

	generateName := getGeneratename(_alert)
	if err := c.alertInterface.CreateAlertWithGenerateName(ctx, c.cfg.PublishNamespace, alert, generateName); err != nil {
		klog.Errorf("fail to create alert %v: %v", _alert, err)
		return err
	}

	return nil
}

type patchStatusValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type patchCountValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value int32  `json:"value"`
}

func (c *AegisController) tryPatchAlertStatus(ctx context.Context, _alert *models.Alert) error {
	fingerprint := _alert.FingerPrint
	if len(fingerprint) == 0 {
		return nil
	}

	req, _ := labels.NewRequirement("fingerprint", selection.Equals, []string{fingerprint})
	selector := labels.NewSelector()
	selector = selector.Add(*req)

	patches := []patchStatusValue{{
		Op:    "replace",
		Path:  "/status/status",
		Value: _alert.Status,
	}}
	patchBytes, _ := json.Marshal(patches)
	if err := c.alertInterface.PatchAlertWithLabelSelector(ctx, c.cfg.PublishNamespace, selector, patchBytes); err != nil {
		klog.Errorf("fail to patch alert %v: %v", _alert, err)
		return err
	}

	return nil
}

func (c *AegisController) tryFetchInCompletedAlerts(ctx context.Context, _alert *models.Alert) ([]*alertv1alpha1.AegisAlert, error) {
	fingerprint := _alert.FingerPrint
	if len(fingerprint) == 0 {
		return nil, nil
	}

	req, _ := labels.NewRequirement("fingerprint", selection.Equals, []string{fingerprint})
	selector := labels.NewSelector()
	selector = selector.Add(*req)

	alerts, err := c.alertInterface.ListAlertWithLabelSelector(ctx, c.cfg.PublishNamespace, selector)
	if err != nil {
		klog.Errorf("fail to list alert with label selector %v: %v", selector, err)
		return nil, err
	}

	todos := make([]*alertv1alpha1.AegisAlert, 0)
	for _, alert := range alerts {
		if alert.Status.OpsStatus.Status != alertv1alpha1.OpsStatusSucceeded && alert.Status.OpsStatus.Status != alertv1alpha1.OpsStatusFailed {
			todos = append(todos, alert)
		}
	}

	klog.V(2).Infof("list %d todo alerts for fingerprint %v", len(todos), fingerprint)

	return todos, nil
}

func (c *AegisController) incurAlertCount(ctx context.Context, todo *alertv1alpha1.AegisAlert) error {
	patches := []patchCountValue{{
		Op:    "replace",
		Path:  "/status/count",
		Value: todo.Status.Count + 1,
	}}
	patchBytes, _ := json.Marshal(patches)
	if err := c.alertInterface.PatchAlert(ctx, c.cfg.PublishNamespace, todo.Name, patchBytes); err != nil {
		klog.Errorf("fail to patch alert %v: %v", todo, err)
		return err
	}

	return nil
}
