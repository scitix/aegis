package rule

import (
	"github.com/scitix/aegis/cli/config"
	"github.com/spf13/cobra"
)

func NewCommand(config *config.AegisCliConfig) *cobra.Command {
	c := &cobra.Command{
		Use:   "rule",
		Short: "Manage with aegis rule",
		Long:  "Manage with aegis rule",
	}

	c.AddCommand(
		NewCreateCommand(config, "create"),
		NewDeleteCmd(config, "delete"),
		NewGetCmd(config, "get"),
	)

	return c
}
