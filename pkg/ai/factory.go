package ai

import (
	"errors"

	kai "github.com/k8sgpt-ai/k8sgpt/pkg/ai"
	"github.com/k8sgpt-ai/k8sgpt/pkg/util"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

type AIProviderFactory interface {
	Load(name string, httpHeaders []string) (kai.IAI, string, error)
}

type DefaultFactory struct{}

func (f *DefaultFactory) Load(name string, headers []string) (kai.IAI, string, error) {
	var configAI kai.AIConfiguration
	if err := viper.UnmarshalKey("ai", &configAI); err != nil {
		return nil, "", err
	}
	klog.V(4).Infof("Loaded AI config: %+v", configAI)

	if len(configAI.Providers) == 0 {
		return nil, "", errors.New("AI providers not defined")
	}

	if name == "" && configAI.DefaultProvider != "" {
		name = configAI.DefaultProvider
	}
	if name == "" {
		name = "openai"
	}

	var aiProvider kai.AIProvider
	for _, p := range configAI.Providers {
		if p.Name == name {
			aiProvider = p
			break
		}
	}

	if aiProvider.Name == "" {
		return nil, "", errors.New("AI provider not found")
	}

	aiProvider.CustomHeaders = util.NewHeaders(headers)
	aiClient := kai.NewClient(aiProvider.Name)

	if err := aiClient.Configure(&aiProvider); err != nil {
		return nil, "", err
	}

	return aiClient, aiProvider.Name, nil
}
