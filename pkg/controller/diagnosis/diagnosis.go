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
	"k8s.io/klog/v2"

	diagnosisv1alpha1 "github.com/scitix/aegis/pkg/apis/diagnosis/v1alpha1"
)

type Diagnosis struct {
	Client         *kubernetes.Client
	Language       string
	CollectorImage string
	EnableProm     bool
	AIClient       kai.IAI
	AIFactory      ai.AIProviderFactory
	Cache          *cache.Cache
	NoCache        bool
	Explain        bool
	// MaxConcurrency     int
	AIProvider string // The name of the AI Provider used for diagnose
	// WithDoc    bool

	AnalyzerMap map[string]common.IAnalyzer
}

func NewDiagnosis(
	kubeClient *kubernetes.Client,
	backend string,
	language string,
	collector_image string,
	enable_prom bool,
	noCache bool,
	explain bool,
	// withDoc bool,
	httpHeaders []string,
) (*Diagnosis, error) {
	c := cache.New(10*time.Minute, 20*time.Minute)

	a := &Diagnosis{
		Client:         kubeClient,
		Language:       language,
		CollectorImage: collector_image,
		EnableProm:     enable_prom,
		Explain:        explain,
		Cache:          c,
		NoCache:        noCache,
		AIFactory:      &ai.DefaultFactory{},
	}

	a.InitAnalyzerMap()

	if !explain {
		// Return early if AI use was not requested.
		return a, nil
	}

	AIClient, AIProvider, err := a.AIFactory.Load(backend, httpHeaders)
	if err != nil {
		return nil, err
	}

	a.AIClient = AIClient
	a.AIProvider = AIProvider
	return a, nil
}

func (d *Diagnosis) RunDiagnosis(ctx context.Context, kind, namespace, name string) (*diagnosisv1alpha1.DiagnosisResult, string, error) {
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
	}

	analyzer, ok := d.AnalyzerMap[kind]
	if !ok {
		return nil, "", fmt.Errorf("Unsupported diagnosis type: %s", kind)
	}

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

	// if not enable ai explain
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

func (d *Diagnosis) InitAnalyzerMap() {
	customAnalyzerMap := make(map[string]common.IAnalyzer)

	customAnalyzerMap["Pod"] = analyzer.NewPodAnalyzer(d.EnableProm)
	customAnalyzerMap["Node"] = analyzer.NewNodeAnalyzer(d.EnableProm)

	d.AnalyzerMap = customAnalyzerMap
}
