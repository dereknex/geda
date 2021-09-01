package app

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"kubeease.com/kubeease/geda/cmd/geda/app/provision"
)

var global struct {
	Verbosity int
}

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "geda",
		Short: "bootstrap kubernetes clusters",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if global.Verbosity > 0 {
				log.SetLevel(log.DebugLevel)
			}
		},
	}
	cmd.AddCommand(provision.NewCommand())
	return cmd
}
