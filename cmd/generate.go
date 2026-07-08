package cmd

import (
	"fmt"

	"github.com/sajadweb/nika-cli/internal"
	"github.com/spf13/cobra"
)

// generateCmd is the parent command: `nika generate` / `nika g`.
var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"g"},
	Short:   "Generate code for a Nika project",
	Long: `Generate controllers, services, DTOs, or full resources for a Nika project.

Available types:
  res (r)        Generate everything (schema + dto + controller + service + module)
  controller (c) Generate only the controller
  service (s)    Generate only the services
  dto (d)        Generate only the DTOs

Usage:
  nika generate <type> <module>
  nika g <type> <module>

Examples:
  nika g res user
  nika g controller product
  nika g c product
  nika g dto order`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		genType := internal.ParseGenerateType(args[0])
		if genType == "" {
			return fmt.Errorf("unknown generate type %q — valid: res (r), controller (c), service (s), dto (d)", args[0])
		}

		return internal.RunGenerate(&internal.GenerateConfig{
			Type:   genType,
			Module: args[1],
		})
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
