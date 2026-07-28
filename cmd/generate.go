package cmd

import (
	"fmt"

	"github.com/nika-framework/nika-cli/internal"
	"github.com/spf13/cobra"
)

// generateCmd is the parent command: `nika generate` / `nika g`.
var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"g"},
	Short:   "Generate code for a Nika project",
	Long: `Generate controllers, services, DTOs, resources, migrations, or seeds.

Available types:
  res (r)         Generate everything (schema + dto + controller + service + module)
  controller (c)  Generate only the controller
  service (s)     Generate only the services
  dto (d)         Generate only the DTOs
  response (rs)   Generate only the responses
  migration (m)   Generate a new database migration
  seed            Generate a new database seed

Usage:
  nika generate <type> <module>
  nika g <type> <module>

Examples:
  nika g res user
  nika g controller product
  nika g c product
  nika g dto order
  nika g migration create_users -d postgres

Microservice workspaces:
  When the project has an apps/ directory, the CLI asks which service the
  module belongs to and generates into that service's src folder, with import
  paths to match. Use -a/--app to answer up front:

  nika g res user -a api
  nika g res order --app micro-grpc
  nika g migration add_index_orders -d mongodb --format go
  nika g seed initial_admins -d postgres

Generating a migration from an existing model (real CREATE TABLE, not a stub):
  nika g migration create_users -d sqlite -m src/user/entity/user.model.go
  nika g migration create_users -d sqlite -m src/user/entity/user.model.go --format sql
  nika g seed initial_users -d sqlite -m src/user/entity/user.model.go`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		rawType := args[0]
		name := args[1]

		// Migration / seed live in a slightly different pipeline.
		if isMigrationType(rawType) {
			db := internal.ParseDatabaseType(generateDatabase)
			if db == "" {
				return fmt.Errorf("migration requires -d/--database (mongodb, postgres, mysql, sqlite)")
			}
			paths, err := internal.GenerateMigration(&internal.MigrationConfig{
				Name:     name,
				Database: db,
				Format:   generateFormat,
				Model:    generateModel,
				TypeName: generateTypeName,
				Table:    generateTable,
			})
			if err != nil {
				return err
			}
			for _, p := range paths {
				fmt.Println("created", p)
			}
			return nil
		}

		if isSeedType(rawType) {
			db := internal.ParseDatabaseType(generateDatabase)
			if db == "" {
				return fmt.Errorf("seed requires -d/--database (mongodb, postgres, mysql, sqlite)")
			}
			path, err := internal.GenerateSeed(&internal.SeedConfig{
				Name:     name,
				Database: db,
				Model:    generateModel,
				TypeName: generateTypeName,
				Table:    generateTable,
			})
			if err != nil {
				return err
			}
			fmt.Println("created", path)
			return nil
		}

		genType := internal.ParseGenerateType(rawType)
		if genType == "" {
			return fmt.Errorf("unknown generate type %q — valid: res (r), controller (c), service (s), dto (d), response (rs), migration (m), seed", rawType)
		}

		return internal.RunGenerate(&internal.GenerateConfig{
			Type:     genType,
			Module:   name,
			Database: generateDatabase,
			App:      generateApp,
		})
	},
}

var (
	generateDatabase string
	generateFormat   string
	generateModel    string
	generateTypeName string
	generateTable    string
	generateApp      string
)

func isMigrationType(s string) bool {
	switch s {
	case "migration", "migrations", "m":
		return true
	}
	return false
}

func isSeedType(s string) bool {
	switch s {
	case "seed", "seeds":
		return true
	}
	return false
}

func init() {
	generateCmd.Flags().StringVarP(&generateDatabase, "database", "d", "", "Database: mongodb, postgres, mysql, or sqlite")
	generateCmd.Flags().StringVarP(&generateFormat, "format", "f", "go", "Migration format for SQL backends: go or sql")
	generateCmd.Flags().StringVarP(&generateModel, "model", "m", "", "Path to a Go model file — generates real DDL/seed data from its db tags")
	generateCmd.Flags().StringVar(&generateTypeName, "type", "", "Struct name inside --model (only needed when the file has several)")
	generateCmd.Flags().StringVar(&generateTable, "table", "", "Override the derived table/collection name")
	generateCmd.Flags().StringVarP(&generateApp, "app", "a", "", "Which app/microservice to generate into (skips the prompt in a workspace)")
	rootCmd.AddCommand(generateCmd)
}
