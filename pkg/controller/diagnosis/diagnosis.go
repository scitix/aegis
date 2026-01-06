package diagnosis

import (
	"context"
	"fmt"
	"time"

	kai "github.com/k8sgpt-ai/k8sgpt/pkg/ai"
	kcommon "github.com/k8sgpt-ai/k8sgpt/pkg/common"
	"github.com/k8sgpt-ai/k8sgpt/pkg/kubernetes"
	cache "github.com/patrickmn/go-cache"
	"github.com/scitix/aegis/pkg/ai"
	"github.com/scitix/aegis/pkg/analyzer"
	"github.com/scitix/aegis/pkg/analyzer/common"
	diagnosisv1alpha1 "github.com/scitix/aegis/pkg/apis/diagnosis/v1alpha1"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"

	kfclientset "github.com/kubeflow/training-operator/pkg/client/clientset/versioned"
)

type Diagnosis struct {
	Client           *kubernetes.Client
	PytorchJobClient kfclientset.Interface
	Language         string
	CollectorImage   string
	EnableProm       bool
	Prometheus       *prom.PromAPI
	AIClient         kai.IAI
	AIFactory        ai.AIProviderFactory
	Cache            *cache.Cache
	NoCache          bool
	Explain          bool
	AIProvider       string
	AnalyzerFactory  map[string]func(*diagnosisv1alpha1.AegisDiagnosis) common.IAnalyzer
}

func NewDiagnosis(
	kubeClient *kubernetes.Client,
	ptClient kfclientset.Interface,
	backend string,
	language string,
	collectorImage string,
	enableProm bool,
	prometheus *prom.PromAPI,
	noCache bool,
	explain bool,
	httpHeaders []string,
) (*Diagnosis, error) {
	c := cache.New(10*time.Minute, 20*time.Minute)
	d := &Diagnosis{
		Client:           kubeClient,
		PytorchJobClient: ptClient,
		Language:         language,
		CollectorImage:   collectorImage,
		EnableProm:       enableProm,
		Prometheus:       prometheus,
		Explain:          explain,
		Cache:            c,
		NoCache:          noCache,
		AIFactory:        &ai.DefaultFactory{},
	}

	d.AnalyzerFactory = map[string]func(*diagnosisv1alpha1.AegisDiagnosis) common.IAnalyzer{
		"Pod": func(_ *diagnosisv1alpha1.AegisDiagnosis) common.IAnalyzer {
			return analyzer.NewPodAnalyzer(d.Prometheus)
		},
		"Node": func(diag *diagnosisv1alpha1.AegisDiagnosis) common.IAnalyzer {
			return analyzer.NewNodeAnalyzer(d.Prometheus)
		},
		"PytorchJob": func(_ *diagnosisv1alpha1.AegisDiagnosis) common.IAnalyzer {
			return analyzer.NewPytorchJobAnalyzer(d.Prometheus, d.PytorchJobClient)
		},
	}

	if explain {
		AIClient, AIProvider, err := d.AIFactory.Load(backend, httpHeaders)
		if err != nil {
			return nil, err
		}
		d.AIClient = AIClient
		d.AIProvider = AIProvider
	}

	return d, nil
}

func (d *Diagnosis) RunDiagnosis(ctx context.Context, diagnosis *diagnosisv1alpha1.AegisDiagnosis) (*diagnosisv1alpha1.DiagnosisResult, string, error) {
	object := diagnosis.Spec.Object
	kind := string(object.Kind)
	name := object.Name
	namespace := object.Namespace

	a := common.Analyzer{
		Analyzer: kcommon.Analyzer{
			Client:    d.Client,
			Context:   ctx,
			AIClient:  d.AIClient,
			Namespace: namespace,
		},
		Name:           name,
		CollectorImage: d.CollectorImage,
		EnableProm:     d.EnableProm,
		Owner:          diagnosis,
	}

	factory, ok := d.AnalyzerFactory[kind]
	if !ok {
		return nil, "", fmt.Errorf("Unsupported diagnosis type: %s", kind)
	}
	analyzer := factory(diagnosis)

	result, err := analyzer.Analyze(a)
	if err != nil {
		return nil, "", fmt.Errorf("Error running analyzer: %s", err)
	}

	failures, warnings, infos := make([]string, 0), make([]string, 0), make([]string, 0)
	for _, v := range result.Error {
		failures = append(failures, v.Text)
	}
	for _, v := range result.Warning {
		warnings = append(warnings, v.Text)
	}
	for _, v := range result.Info {
		infos = append(infos, v.Text)
	}

	dresult := &diagnosisv1alpha1.DiagnosisResult{
		Failures: failures,
		Warnings: warnings,
		Infos:    infos,
	}

	if !d.Explain {
		return dresult, "", nil
	}

	inputKey := fmt.Sprintf("%s/%s/%s/%s", d.Language, kind, namespace, name)
	if explain, found := d.Cache.Get(inputKey); found {
		klog.V(4).Infof("Get explain from cache for %s", inputKey)
		return dresult, explain.(string), nil
	}

	prompt := analyzer.Prompt(result)
	if prompt == "" {
		klog.Info("Do not need to get explain")
		return dresult, "Healthy: Yes", nil
	}

	klog.V(4).Infof("Prompt: %s", prompt)
	response, err := d.AIClient.GetCompletion(ctx, prompt)
	if err != nil {
		klog.Errorf("Failed to get AI completion: %v", err)
		return dresult, "", fmt.Errorf("failed to get explain: %v", err)
	}

	if !d.NoCache {
		d.Cache.Set(inputKey, response, 10*time.Minute)
	}

	return dresult, response, nil
}
