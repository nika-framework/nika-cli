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

  nika start                       # the default app
  nika start --watch -a api        # one specific service
  nika start --watch -a grpc-micro # another one
  nika start --watch -a            # every service at once, one process each
  nika start --watch               # restart on file changes
  nika start ./apps/api/main.go

With -a and no name, every app starts together: output is tagged with the
service it came from, a change under apps/<name>/ restarts only that service,
and a change to shared code restarts all of them.`,
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
	startCmd.Flags().StringVarP(&startApp, "app", "a", "",
		"Which app/microservice to start; pass -a with no name to start them all (default: default_app in .nika.toml)")
	// Giving -a an optional value is what makes `nika start -a` mean "all of
	// them". pflag then hands "-a api" back as the marker plus a positional
	// argument, which StartApp.resolve untangles.
	startCmd.Flags().Lookup("app").NoOptDefVal = start.AllApps
	rootCmd.AddCommand(startCmd)
}
