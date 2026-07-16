package cmd

import (
	"github.com/nika-framework/nika-cli/internal"
	"github.com/spf13/cobra"
)

// agentCmd represents the agent command which configures AI files for the project.
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Configure AI agent files for Nika project generation",
	Long: `Creates custom instructions, prompts, and agent configurations 
inside the .github directory of the current project to enable AI tools 
(like Copilot or Cursor) to easily generate new Nika modules with fields automatically.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return internal.SetupAgent()
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
}
