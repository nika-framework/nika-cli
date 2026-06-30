package cmd

import (
	"fmt"

	"github.com/sajadweb/nika-cli/internal"
	"github.com/spf13/cobra"
)

// newCmd defines the "new" Cobra command that creates a new Nika application.
var newCmd = &cobra.Command{
	Use:   "new [app-name]",
	Short: "Create a new Nika application",
	Long: `Scaffold a new Nika project from the official template.

The command validates the project name, clones the template repository,
customizes module and import paths, and initializes a fresh git repository.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]

		if err := internal.RunNewProject(
			&internal.CreateApp{Name: appName},
			nil, // use real Runner
			nil, // use real FileOps
		); err != nil {
			return fmt.Errorf("failed to create project: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}
