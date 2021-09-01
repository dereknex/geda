package provision

import (
	"github.com/spf13/cobra"
	"kubeease.com/kubeease/geda/cmd/geda/app/bootstrap"
	"kubeease.com/kubeease/geda/cmd/geda/app/provision/preflight"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision infrastructure resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			_, err := bootstrap.StartCluster(ctx)
			if err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return nil
			}
		},
	}
	cmd.AddCommand(preflight.NewCommand())
	return cmd
}
