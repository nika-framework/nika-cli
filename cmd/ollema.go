package cmd

import (
	"strings"

	"github.com/nika-framework/nika-cli/internal"
	"github.com/spf13/cobra"
)

// ollemaCmd sends a single prompt to a local Ollama instance.
var ollemaCmd = &cobra.Command{
	Use:   "ollema <model> <text>",
	Short: "Send text to Ollama or generate a module from a prompt",
	Long: `Send a prompt to Ollama running on the local machine and print its response.

When the prompt asks to create/build a module, the response is converted into
a validated resource definition and generated with Nika's existing templates.

The Ollama endpoint defaults to http://localhost:11434. Set OLLAMA_HOST to use
a different local endpoint. The text should be quoted when it contains spaces.

Example:
  nika ollema llama3.2 "Explain dependency injection in Go"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if isRoutePrompt(args[1]) {
			return internal.RunOllemaRoute(args[0], args[1], cmd.OutOrStdout())
		}
		if isModulePrompt(args[1]) {
			return internal.RunOllemaModule(args[0], args[1], cmd.OutOrStdout())
		}
		return internal.RunOllema(args[0], args[1], cmd.OutOrStdout())
	},
}

func isRoutePrompt(prompt string) bool {
	prompt = strings.ToLower(prompt)
	for _, marker := range []string{"route", "روت", "mock", "ماک", "دیتای ماک", "endpoint"} {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

func isModulePrompt(prompt string) bool {
	prompt = strings.ToLower(prompt)
	for _, marker := range []string{"module", "ماژول", "بساز", "ساخت", "build", "create", "generate"} {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(ollemaCmd)
}
