package selfhealing

import (
	"flag"

	"github.com/scitix/aegis/selfhealing/config"
	"github.com/scitix/aegis/selfhealing/node"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

func NewCommand(name string) *cobra.Command {
	f := config.LoadConfig()
	c := &cobra.Command{
		Use:   name,
		Short: "Cli for Aegis self-healing system",
		Long:  "Cli for Aegis self-healing system",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if err := f.Complete(); err != nil {
				klog.Fatalf("Complete config failed: %v", err)
			}
		},
	}

	c.PersistentFlags().StringVar(&f.Kubeconfig, "kubeconfig", "", "Path to the kubeconfig file.")
	c.PersistentFlags().StringVar(&f.Region, "region", "", "cloud region")
	c.PersistentFlags().StringVar(&f.OrgName, "orgname", "", "cloud orgname")
	c.PersistentFlags().StringVar(&f.ClusterName, "clustername", "", "kubernetes cluster name")
	c.PersistentFlags().BoolVar(&f.EnableLeaderElection, "enable-leader-election", true, "Enable leader election in a kubernetes cluster.")

	c.AddCommand(
		node.NewCommand(f, "node"),
	)

	// init add the klog flags
	klog.InitFlags(flag.CommandLine)
	c.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	return c
}
