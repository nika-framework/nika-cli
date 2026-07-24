package cmd

import (
	"github.com/nika-framework/nika-cli/internal"
	"github.com/spf13/cobra"
)

// seedCmd invokes ./cmd/seed in the user's project.
var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Run database seeders for the current Nika project",
	Long: `Run data seeds against the project's configured database.

This command shells out to ./cmd/seed in the current project.

Subcommands:
  run             Run every pending seed (respects AlwaysRun)
  run NAME…       Run only the named seeds regardless of prior state
  status          Print applied vs. pending seeds`,
}

var seedRunCmd = &cobra.Command{
	Use:   "run [names...]",
	Short: "Run pending seeds, or just the named ones",
	RunE: func(cmd *cobra.Command, args []string) error {
		return internal.RunSeed("run", args...)
	},
}

var seedStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show seed status",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return internal.RunSeed("status")
	},
}

func init() {
	seedCmd.AddCommand(seedRunCmd)
	seedCmd.AddCommand(seedStatusCmd)
	rootCmd.AddCommand(seedCmd)
}
