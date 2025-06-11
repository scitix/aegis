package diagnosis

import (
	"context"
	"errors"
	"fmt"
	"time"

	kai "github.com/k8sgpt-ai/k8sgpt/pkg/ai"
	kcommon "github.com/k8sgpt-ai/k8sgpt/pkg/common"
	"github.com/k8sgpt-ai/k8sgpt/pkg/kubernetes"
	"github.com/k8sgpt-ai/k8sgpt/pkg/util"
	cache "github.com/patrickmn/go-cache"
	"github.com/spf13/viper"
	"gitlab.scitix-inner.ai/k8s/aegis/pkg/analyzer"
	"gitlab.scitix-inner.ai/k8s/aegis/pkg/analyzer/common"
	"k8s.io/klog/v2"

	diagnosisv1alpha1 "gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/diagnosis/v1alpha1"
)

type Diagnosis struct {
	Client   *kubernetes.Client
	Language string
	AIClient kai.IAI
	Cache    *cache.Cache
	NoCache  bool
	Explain  bool
	// MaxConcurrency     int
	AIProvider string // The name of the AI Provider used for diagnose
	// WithDoc    bool

	AnalyzerMap map[string]common.IAnalyzer
}

func NewDiagnosis(
	backend string,
	language string,
	noCache bool,
	explain bool,
	// withDoc bool,
	httpHeaders []string,
) (*Diagnosis, error) {
	// Get kubernetes client from viper.
	kubecontext := viper.GetString("kubecontext")
	kubeconfig := viper.GetString("kubeconfig")
	client, err := kubernetes.NewClient(kubecontext, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("initialising kubernetes client: %w", err)
	}

	c := cache.New(10*time.Minute, 20*time.Minute)

	a := &Diagnosis{
		Client:   client,
		Language: language,
		Explain:  explain,
		Cache:    c,
		NoCache:  noCache,
		// WithDoc:  withDoc,
		AnalyzerMap: analyzer.GetAnalyzerMap(),
	}

	if !explain {
		// Return early if AI use was not requested.
		return a, nil
	}

	var configAI kai.AIConfiguration
	if err := viper.UnmarshalKey("ai", &configAI); err != nil {
		return nil, err
	}

	if len(configAI.Providers) == 0 {
		return nil, errors.New("AI provider not specified in configuration.")
	}

	// Backend string will have high priority than a default provider
	// Hence, use the default provider only if the backend is not specified by the user.
	if configAI.DefaultProvider != "" && backend == "" {
		backend = configAI.DefaultProvider
	}

	if backend == "" {
		backend = "openai"
	}

	var aiProvider kai.AIProvider
	for _, provider := range configAI.Providers {
		if backend == provider.Name {
			aiProvider = provider
			break
		}
	}

	if aiProvider.Name == "" {
		return nil, fmt.Errorf("AI provider %s not specified in configuration. Please run k8sgpt auth", backend)
	}

	aiClient := kai.NewClient(aiProvider.Name)
	customHeaders := util.NewHeaders(httpHeaders)
	aiProvider.CustomHeaders = customHeaders
	if err := aiClient.Configure(&aiProvider); err != nil {
		return nil, err
	}
	a.AIClient = aiClient
	a.AIProvider = aiProvider.Name
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
		Name: name,
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

// func (d *Diagnose) GetAIResult(ctx context.Context, result *common.Result, kind string) (string, error) {
// 	promptTemplate := ai.PromptMap["default"]
// 	if prompt, ok := ai.PromptMap[kind]; ok {
// 		promptTemplate = prompt
// 	}

// 	failureText := strings.Join(result.Failures, "  ")
// 	warningText := strings.Join(result.Warnings, "  ")
// 	infoText := strings.Join(result.Infos, "  ")

// 	// Process template.
// 	prompt := fmt.Sprintf(strings.TrimSpace(promptTemplate), failureText, warningText, infoText)
// 	prompt :=
// 	klog.V(4).Infof("Prompt: %s", prompt)

// 	response, err := d.AIClient.GetCompletion(ctx, prompt)
// 	if err != nil {
// 		klog.Errorf("Failed to get AI completion: %v", err)
// 		return nil, fmt.Errorf("failed to get explain")
// 	}

// 	return &response, nil
// }
