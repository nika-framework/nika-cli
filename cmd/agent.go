package cmd

import (
	"os"
	"strings"

	"github.com/nika-framework/nika-cli/internal"
	"github.com/nika-framework/nika-cli/internal/aiagent"
	"github.com/spf13/cobra"
)

var (
	agentReadOnly   bool
	agentAnyCommand bool
	agentQuiet      bool
	agentDir        string

	agentPort int
	agentHost string
	agentOpen bool
)

// agentCmd runs the AI agent, or installs the editor AI files when asked.
var agentCmd = &cobra.Command{
	Use:   "agent [prompt]",
	Short: "Run the AI agent on this project",
	Long: `Run an instruction against the current project using the provider configured
in .nika.toml.

The agent reads and edits files, scaffolds modules with Nika's own templates,
and runs builds to check its work — so any instruction works, not just module
generation:

  nika agent "add a price field (float64) to the product model"
  nika agent "generate a category module with name and slug for sqlite"
  nika agent "why does the user controller return 500 on update?"

Run it with no arguments for an interactive session, "nika agent start" for a
chat page in the browser, or "nika agent files" to install the editor AI files.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		options := agentRunOptions()
		if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
			return aiagent.RunPrompt(cmd.Context(), args[0], cmd.OutOrStdout(), options)
		}
		return aiagent.RunInteractive(cmd.Context(), os.Stdin, cmd.OutOrStdout(), options)
	},
}

var agentInitCmd = &cobra.Command{
	Use:   "init <provider>",
	Short: "Configure an AI provider in .nika.toml",
	Long: `Configure the AI provider used by "nika agent".

Supported providers:
  ollama    a local Ollama instance (no API key)
  chatgpt   the OpenAI API        (OPENAI_API_KEY)
  9router   OpenRouter            (OPENROUTER_API_KEY)
  claude    the Anthropic API     (ANTHROPIC_API_KEY)

The API key itself is never written to .nika.toml — only the name of the
environment variable to read it from.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := agentDir
		if strings.TrimSpace(dir) == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			dir = cwd
		}
		return aiagent.InitProvider(dir, args[0], cmd.OutOrStdout())
	},
}

var agentModelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List installed Ollama models and which of them can call tools",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return aiagent.ListModels(cmd.OutOrStdout(), "")
	},
}

var agentStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Open a chat page in the browser that edits this project",
	Long: `Start a local chat server and open it in the browser.

Anything typed in the chat is carried out in the directory the command was
started from: the agent reads, edits and generates files there, and streams
each tool call back to the page as it happens.

The server binds to 127.0.0.1 and requires a per-run token embedded in the URL
it prints, because it holds write access to your source tree.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := agentDir
		if strings.TrimSpace(dir) == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			dir = cwd
		}
		server, err := aiagent.NewServer(aiagent.ServerOptions{
			Dir:             dir,
			Port:            agentPort,
			Host:            agentHost,
			Open:            agentOpen,
			ReadOnly:        agentReadOnly,
			AllowAnyCommand: agentAnyCommand,
		})
		if err != nil {
			return err
		}
		return server.ListenAndServe()
	},
}

var agentFilesCmd = &cobra.Command{
	Use:     "files",
	Aliases: []string{"setup"},
	Short:   "Install Nika's instructions for editor AI agents (Copilot, Cursor)",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return internal.SetupAgent()
	},
}

func agentRunOptions() aiagent.RunOptions {
	return aiagent.RunOptions{
		Dir:             agentDir,
		ReadOnly:        agentReadOnly,
		AllowAnyCommand: agentAnyCommand,
		Quiet:           agentQuiet,
	}
}

func init() {
	agentCmd.PersistentFlags().BoolVar(&agentReadOnly, "read-only", false, "Let the agent inspect the project but never change it")
	agentCmd.PersistentFlags().BoolVar(&agentAnyCommand, "allow-any-command", false, "Remove the run_command allowlist (use with care)")
	agentCmd.PersistentFlags().StringVarP(&agentDir, "dir", "C", "", "Project directory to work in (default: current directory)")
	agentCmd.Flags().BoolVarP(&agentQuiet, "quiet", "q", false, "Print only the final answer, not the tool trace")

	agentStartCmd.Flags().IntVarP(&agentPort, "port", "p", 7777, "Port to listen on (0 picks a free port)")
	agentStartCmd.Flags().StringVar(&agentHost, "host", "127.0.0.1", "Address to bind")
	agentStartCmd.Flags().BoolVar(&agentOpen, "open", true, "Open the chat page in the default browser")

	agentCmd.AddCommand(agentInitCmd)
	agentCmd.AddCommand(agentModelsCmd)
	agentCmd.AddCommand(agentStartCmd)
	agentCmd.AddCommand(agentFilesCmd)
	rootCmd.AddCommand(agentCmd)
}
