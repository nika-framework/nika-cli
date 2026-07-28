package cmd

import (
	"fmt"

	"github.com/nika-framework/nika-cli/internal"
	"github.com/spf13/cobra"
)

// appCmd inspects and configures the microservice layout.
var appCmd = &cobra.Command{
	Use:     "app",
	Aliases: []string{"apps", "workspace"},
	Short:   "Inspect and configure the apps in a microservice workspace",
	Long: `Manage the workspace layout recorded in .nika.toml.

A Nika project is either a single application (modules in src/) or a
microservice workspace (one directory per service under apps/, each with its
own src/ and main.go). The CLI detects which from disk; these commands let you
see what it found and pick the default service.`,
}

var appListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the apps in this project",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		workspace, err := internal.LoadWorkspace()
		if err != nil {
			return err
		}
		if workspace.Microservice {
			fmt.Printf("Microservice workspace — module %s\n\n", workspace.ModulePath)
		} else {
			fmt.Printf("Single application — module %s\n\n", workspace.ModulePath)
		}
		for _, app := range workspace.Apps {
			marker := " "
			if app.Name == workspace.DefaultApp {
				marker = "*"
			}
			fmt.Printf(" %s %-16s src: %-24s run: %s\n", marker, app.Name, app.SrcDir, app.RunCommand())
			if modules := workspace.Modules(app); len(modules) > 0 {
				fmt.Printf("   %-16s modules: %v\n", "", modules)
			}
		}
		if workspace.DefaultApp != "" {
			fmt.Printf("\n * = default for `nika start` (change it with `nika app use <name>`)\n")
		}
		return nil
	},
}

var appUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the app that `nika start` runs by default",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workspace, err := internal.LoadWorkspace()
		if err != nil {
			return err
		}
		if err := workspace.SetDefaultApp(args[0]); err != nil {
			return err
		}
		target := workspace.Find(workspace.DefaultApp)
		fmt.Printf("Default app is now %q (%s)\n", workspace.DefaultApp, target.RunCommand())
		return nil
	},
}

var appSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Rewrite .nika.toml to match the apps found on disk",
	Long: `Detect the project layout and write it back to .nika.toml.

Run this after adding or removing a service under apps/. It records the app
list, gives each app a run command, and points [build] at the default app so
"nika start" no longer tries to run "go run ." at a root that has no main
package.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		workspace, err := internal.LoadWorkspace()
		if err != nil {
			return err
		}
		if err := workspace.Sync(); err != nil {
			return err
		}
		fmt.Printf("Updated .nika.toml: %d app(s) — %v\n", len(workspace.Apps), workspace.AppNames())
		if workspace.Config.Workspace.DefaultApp != "" {
			fmt.Printf("Default app: %s → %s\n", workspace.Config.Workspace.DefaultApp, workspace.Config.Build.Cmd)
		}
		return nil
	},
}

func init() {
	appCmd.AddCommand(appListCmd)
	appCmd.AddCommand(appUseCmd)
	appCmd.AddCommand(appSyncCmd)
	rootCmd.AddCommand(appCmd)
}
