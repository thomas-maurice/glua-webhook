package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "glua-webhook",
	Short: "Run Lua scripts as Kubernetes admission webhooks",
	Long: `glua-webhook executes Lua scripts as Kubernetes admission webhooks.

The primary use case is running as a webhook server that processes admission
requests from the Kubernetes API server. Scripts are stored in ConfigMaps and
referenced via annotations on resources.

The 'exec' command allows testing scripts locally before deploying them.`,
}

func init() {
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(webhookCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
