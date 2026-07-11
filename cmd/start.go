package cmd

import (
	"github.com/nika-framework/nika-cli/internal/start"
	"github.com/spf13/cobra"
)

var watchMode bool

// startCmd defines the start command
var startCmd = &cobra.Command{
	Use:   "start [file-or-dir]",
	Short: "Start Nika application",
	Long:  `Runs the Go project in the given directory or file.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := "./main.go" // default when no arg is given

		if len(args) > 0 && args[0] != "" && args[0] != "." {
			target = args[0]
		}

		start := start.NewStartApp(target, watchMode)
		// Normal mode: just run once
		return start.Run()
	},
}

func init() {
	// Define the --watch flag (boolean)
	startCmd.Flags().BoolVar(&watchMode, "watch", false, "Run in watch mode (auto-restart on changes)")
	rootCmd.AddCommand(startCmd)
}
