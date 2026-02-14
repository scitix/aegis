package config

import (
	"github.com/scitix/aegis/internal/k8s"
	alertclientset "github.com/scitix/aegis/pkg/generated/alert/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

type SelfHealingConfig struct {
	Kubeconfig           string
	Region               string
	OrgName              string
	ClusterName          string
	EnableLeaderElection bool
	KubeClient           kubernetes.Interface
	AlertClient          alertclientset.Interface
}

func LoadConfig() *SelfHealingConfig {
	return &SelfHealingConfig{}
}

func (c *SelfHealingConfig) Complete() error {
	cfg, kubeClient, err := k8s.CreateApiserverClient("", c.Kubeconfig)
	if err != nil {
		return err
	}
	c.KubeClient = kubeClient
	c.AlertClient, err = alertclientset.NewForConfig(cfg)
	if err != nil {
		return err
	}

	return nil
}

const TicketSupervisorAegis = "aegis"
