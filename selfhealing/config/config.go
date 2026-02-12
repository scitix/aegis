package config

import (
	"time"

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
	Registry             string
	Repository           string
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

// NodePollerConfig holds the configuration for the active-polling node self-healer.
type NodePollerConfig struct {
	Enabled              bool
	PollInterval         time.Duration // default 10s
	ResyncInterval       time.Duration // default 1h
	CordonResyncInterval time.Duration // default 10min
	MaxAlertsPerRound    int           // default 20
	PriorityConfigMap    string        // default "aegis-priority"
	PriorityNamespace    string        // default "monitoring"
}

const TicketSupervisorAegis = "aegis"
