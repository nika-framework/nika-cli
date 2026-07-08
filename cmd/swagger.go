package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// swaggerCmd is the root command for Swagger operations
var swaggerCmd = &cobra.Command{
	Use:   "swagger",
	Short: "Manage Swagger documentation",
	Long:  `Generate, format, and validate Swagger/OpenAPI documentation using swaggo/swag.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Check if swag is installed; if not, install it
		if err := checkSwagInstalled(); err != nil {
			fmt.Println("🔧 swag not found, installing...")
			if err := installSwag(); err != nil {
				return fmt.Errorf("failed to install swag: %w", err)
			}
			fmt.Println("✅ swag installed successfully.")
		}
		return nil
	},
}

// swaggerInitCmd implements `swagger init`
var swaggerInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Swagger documentation",
	Long:  `Run 'swag init' to generate docs from annotations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Gather flags
		dir, _ := cmd.Flags().GetString("dir")
		output, _ := cmd.Flags().GetString("output")
		parseDependency, _ := cmd.Flags().GetBool("parseDependency")
		parseInternal, _ := cmd.Flags().GetBool("parseInternal")
		parseDepth, _ := cmd.Flags().GetInt("parseDepth")
		instanceName, _ := cmd.Flags().GetString("instanceName")

		// Build arguments for swag
		swagArgs := []string{"init"}
		if dir != "" {
			swagArgs = append(swagArgs, "--dir", dir)
		}
		if output != "" {
			swagArgs = append(swagArgs, "--output", output)
		}
		if parseDependency {
			swagArgs = append(swagArgs, "--parseDependency")
		}
		if parseInternal {
			swagArgs = append(swagArgs, "--parseInternal")
		}
		if parseDepth > 0 {
			swagArgs = append(swagArgs, "--parseDepth", fmt.Sprintf("%d", parseDepth))
		}
		if instanceName != "" {
			swagArgs = append(swagArgs, "--instanceName", instanceName)
		}

		// Execute swag init
		execCmd := exec.Command("swag", swagArgs...)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		if err := execCmd.Run(); err != nil {
			return fmt.Errorf("swag init failed: %w", err)
		}
		return nil
	},
}

// (Optional) Add more subcommands like `fmt`, `gen`, etc.
// Example:
// var swaggerFmtCmd = &cobra.Command{...}

// checkSwagInstalled checks if `swag` command exists
func checkSwagInstalled() error {
	cmd := exec.Command("swag", "--version")
	return cmd.Run()
}

// installSwag installs swag using `go install`
func installSwag() error {
	cmd := exec.Command("go", "install", "github.com/swaggo/swag/cmd/swag@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

var swaggerFmtCmd = &cobra.Command{
	Use:   "fmt",
	Short: "Format Swagger annotations",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		execCmd := exec.Command("swag", "fmt", "--dir", dir)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		return execCmd.Run()
	},
}

func init() {
	// Define flags for `swagger init`
	swaggerInitCmd.Flags().String("dir", "./", "Directory where main.go is located")
	swaggerInitCmd.Flags().String("output", "./docs", "Output directory for generated docs")
	swaggerInitCmd.Flags().Bool("parseDependency", false, "Parse dependency")
	swaggerInitCmd.Flags().Bool("parseInternal", false, "Parse internal packages")
	swaggerInitCmd.Flags().Int("parseDepth", 100, "Depth of parsing")
	swaggerInitCmd.Flags().String("instanceName", "", "Instance name for the docs")

	// Add subcommands to swaggerCmd
	swaggerCmd.AddCommand(swaggerInitCmd)
	// Add more subcommands here if needed: swaggerCmd.AddCommand(swaggerFmtCmd)
	swaggerFmtCmd.Flags().String("dir", "./", "Directory to format")
	swaggerCmd.AddCommand(swaggerFmtCmd)

	// Register swaggerCmd under rootCmd
	rootCmd.AddCommand(swaggerCmd)
}
