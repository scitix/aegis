package cli

import (
	"flag"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/scitix/aegis/cli/auth"
	"github.com/scitix/aegis/cli/config"
	"github.com/scitix/aegis/cli/rule"
)

func NewCommand(name string) *cobra.Command {
	f := config.LoadConfig()
	c := &cobra.Command{
		Use:   name,
		Short: "Cli for Aegis system",
		Long:  "Cli for Aegis system",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if err := f.Complete(); err != nil {
				klog.Fatalf("Complete config failed: %v", err)
			}
		},
	}

	c.PersistentFlags().StringVar(&f.Kubeconfig, "kubeconfig", "", "Path to the kubeconfig file.")
	c.PersistentFlags().StringVar(&f.Registry, "registry", "", "registry endpoint.")
	c.PersistentFlags().BoolVar(&f.Public, "public", false, "Whether is public env or not")

	c.AddCommand(
		auth.NewCommand("aegis", "auth"),
		rule.NewCommand(f),
	)

	// init add the klog flags
	klog.InitFlags(flag.CommandLine)
	c.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	return c
}
