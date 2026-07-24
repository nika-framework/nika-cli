package cmd

import (
	"github.com/nika-framework/nika-cli/internal"
	"github.com/spf13/cobra"
)

// migrateCmd invokes the project's ./cmd/migrate binary. The user's project
// implements the actual runner (using common/sqldb/migration or
// common/mongodb/migration); this command is a thin ergonomic wrapper.
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations for the current Nika project",
	Long: `Run database migrations against the project's configured database.

This command shells out to ./cmd/migrate in the current project. Scaffold that
binary once (or use "nika new") and it will automatically discover Go-based
migrations under internal/database/migrations.

Subcommands:
  up          Apply every pending migration
  up N        Apply the next N pending migrations
  down        Roll back the most recent migration
  down N      Roll back the last N applied migrations
  status      Print applied vs. pending migrations`,
}

var migrateUpCmd = &cobra.Command{
	Use:   "up [n]",
	Short: "Apply pending migrations",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return internal.RunMigrate("up", args...)
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down [n]",
	Short: "Roll back applied migrations (default: 1)",
	Long: `Roll back applied migrations, newest first.

Examples:
  nika migrate down       # roll back the most recent migration
  nika migrate down 3     # roll back the last three migrations`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return internal.RunMigrate("down", args...)
	},
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return internal.RunMigrate("status")
	},
}

func init() {
	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
	rootCmd.AddCommand(migrateCmd)
}
