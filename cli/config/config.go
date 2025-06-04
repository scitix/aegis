package config

import (
	"os"
	"path/filepath"

	"gitlab.scitix-inner.ai/k8s/aegis/internal/k8s"
	ruleclientset "gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/rule/clientset/versioned"
	templateclientset "gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/template/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

type AegisCliConfig struct {
	Kubeconfig string
	Registry   string
	Public     bool

	KubeClient     kubernetes.Interface
	RuleClient     ruleclientset.Interface
	TemplateClient templateclientset.Interface
}

func LoadConfig() *AegisCliConfig {
	return &AegisCliConfig{}
}

func GetKubeConfigPath() string {
	if file := os.Getenv("KUBECONFIG"); file != "" {
		return file
	}

	return filepath.Join(os.Getenv("HOME"), ".kube", "config")
}

func (c *AegisCliConfig) Complete() error {
	cfg, kubeClient, err := k8s.CreateApiserverClient("", c.Kubeconfig)
	if err != nil {
		return err
	}
	c.KubeClient = kubeClient

	c.TemplateClient, err = templateclientset.NewForConfig(cfg)
	if err != nil {
		return err
	}

	c.RuleClient, err = ruleclientset.NewForConfig(cfg)
	return err
}

func (c *AegisCliConfig) GetRegistryAddress() string {
	return c.Registry
}
