package cmd

import (
	"github.com/nika-framework/nika-cli/internal"
	"github.com/spf13/cobra"
)

// agentCmd represents the agent command which configures AI files for the project.
var agentCmd = &cobra.Command{
	Use:   "agent [prompt]",
	Short: "Run the configured AI agent or configure agent files",
	Long: `Run an AI prompt using the provider configured in .nika.toml.

Use "nika agent init <provider>" to configure a provider, or run "nika agent"
without a prompt to install Nika's project instructions and AI files.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return internal.RunConfiguredAgent(args[0], cmd.OutOrStdout())
		}
		return internal.SetupAgent()
	},
}

var agentInitCmd = &cobra.Command{
	Use:   "init <provider>",
	Short: "Configure an AI provider in .nika.toml",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return internal.InitAgent(args[0], cmd.OutOrStdout())
	},
}

func init() {
	agentCmd.AddCommand(agentInitCmd)
	rootCmd.AddCommand(agentCmd)
}
