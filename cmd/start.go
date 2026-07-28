package cmd

import (
	"github.com/nika-framework/nika-cli/internal/start"
	"github.com/spf13/cobra"
)

var (
	watchMode bool
	startApp  string
)

// startCmd defines the start command
var startCmd = &cobra.Command{
	Use:   "start [file-or-dir]",
	Short: "Start Nika application",
	Long: `Run the project.

With no arguments the command uses the [build] cmd from .nika.toml. In a
microservice workspace it runs the default app — or the one given with
-a/--app — so you do not have to remember each service's main.go path:

  nika start                 # the default app
  nika start -a micro-grpc   # one specific service
  nika start --watch         # restart on file changes
  nika start ./apps/api/main.go`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := ""
		if len(args) > 0 {
			target = args[0]
		}
		return start.NewStartAppFor(target, startApp, watchMode).Run()
	},
}

func init() {
	startCmd.Flags().BoolVar(&watchMode, "watch", false, "Run in watch mode (auto-restart on changes)")
	startCmd.Flags().StringVarP(&startApp, "app", "a", "", "Which app/microservice to start (default: default_app in .nika.toml)")
	rootCmd.AddCommand(startCmd)
}
