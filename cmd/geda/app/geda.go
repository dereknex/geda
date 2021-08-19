package app

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	"kubeease.com/kubeease/geda/pkg/log"
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
				log.SetLevel(zapcore.DebugLevel)
			}
		},
	}
	return cmd
}
