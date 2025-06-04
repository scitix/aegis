package diagnosis

import (
	"context"
	"testing"

	"github.com/k8sgpt-ai/k8sgpt/pkg/ai"
	"github.com/spf13/viper"
)

func TestCreateAIClient(t *testing.T) {
	backend := "openai"
	viper.SetConfigFile("/scratch/vault/home/lzhang02/aegis/config.yaml")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("unable to load config.yaml: %v", err)
	}
	t.Logf("%+v", viper.AllKeys())

	var configAI ai.AIConfiguration
	if err := viper.UnmarshalKey("ai", &configAI); err != nil {
		t.Fatalf("unable to load config.yml: %v", err)
	}

	if len(configAI.Providers) == 0 {
		t.Fatalf("no providers found")
	}

	var aiProvider ai.AIProvider
	for _, provider := range configAI.Providers {
		if backend == provider.Name {
			aiProvider = provider
			break
		}
	}

	aiClient := ai.NewClient(aiProvider.Name)
	if err := aiClient.Configure(&aiProvider); err != nil {
		t.Fatalf("unable to configure ai client: %v", err)
	}

	response, err := aiClient.GetCompletion(context.Background(), "who are you!")
	if err != nil {
		t.Fatalf("unable to get completion: %v", err)
	}

	t.Logf("response: %v", response)
}