package cmd

import (
	"github.com/spf13/cobra"

	cfg "github.com/secret2830/chainlink-integration-monitor/config"
	"github.com/secret2830/chainlink-integration-monitor/daemon"
)

func GetRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "chainlink-integration-monitor",
		Short:   "Start daemon for monitoring the chainlink integration",
		Example: `chainlink-integration-monitor [config-file]`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var configFileName string

			if len(args) == 0 {
				configFileName = cfg.DefaultConfigFileName
			} else {
				configFileName = args[0]
			}

			config, err := cfg.LoadYAMLConfig(configFileName)
			if err != nil {
				return err
			}

			app := daemon.NewApplication(config)
			app.Start()

			return nil
		},
	}

	return cmd
}
