package provision

import (
	"github.com/spf13/cobra"
	"kubeease.com/kubeease/geda/cmd/geda/app/provision/preflight"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision infrastructure resources",
	}
	cmd.AddCommand(preflight.NewCommand())
	return cmd
}
