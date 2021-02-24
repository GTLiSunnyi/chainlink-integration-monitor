package main

import (
	"os"

	"github.com/secret2830/chainlink-integration-monitor/cmd"
	"github.com/secret2830/chainlink-integration-monitor/common"
)

func main() {
	rootCmd := cmd.GetRootCmd()

	if err := rootCmd.Execute(); err != nil {
		common.Logger.Error(err)
		os.Exit(1)
	}
}
