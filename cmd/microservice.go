package cmd

import (
	"fmt"
	"strings"

	"github.com/nika-framework/nika-cli/internal"
	"github.com/spf13/cobra"
)

var microInitName string

// microserviceCmd groups the workspace conversion and the per-transport
// scaffolders.
var microserviceCmd = &cobra.Command{
	Use:     "microservice",
	Aliases: []string{"micro", "ms"},
	Short:   "Convert to a microservice workspace and add transport services",
	Long: `Grow a Nika project into several processes.

A new project is one application with its modules in src/. Two commands take it
from there:

  nika microservice init            move src/ and main.go under apps/api/,
                                    rewriting every import that pointed at them

  nika microservice <transport>     add apps/<transport>-micro, wired to that
                                    transport, or apps/<name> when you name it

Transports: ` + strings.Join(internal.MicroTransportNames(), ", ") + `

Every service is an ordinary Nika app — ` + "`nika g res user -a grpc-micro`" + ` generates
into it exactly as it would into an HTTP app, and the same controller can serve
both an HTTP route and a message pattern.`,
}

var microserviceInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Convert this single-application project into a microservice workspace",
	Long: `Move src/ and main.go under apps/<name>/ and rewrite the imports.

The default name is "api". Everything that is shared — go.mod, .env, internal/,
cmd/migrate, cmd/seed — stays at the project root, so migrations and seeds keep
working untouched. .nika.toml is updated to mode = "microservice" with the moved
app as default_app.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return internal.RunMicroserviceInit(&internal.MicroInitConfig{AppName: microInitName})
	},
}

var microserviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the supported microservice transports",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		for _, transport := range internal.MicroTransports() {
			fmt.Printf("  %-8s %-16s %s\n", transport.Name, "apps/"+transport.DefaultAppName(), transport.Title)
			fmt.Printf("  %-8s %s\n", "", transport.Summary)
			if len(transport.Aliases) > 0 {
				fmt.Printf("  %-8s aliases: %s\n", "", strings.Join(transport.Aliases, ", "))
			}
			fmt.Println()
		}
		return nil
	},
}

// newMicroTransportCmd builds the `nika microservice <transport> [name]`
// sub-command for one entry in the catalogue.
func newMicroTransportCmd(transport internal.MicroTransport) *cobra.Command {
	var envLines strings.Builder
	for _, variable := range transport.Env(transport.DefaultAppName()) {
		fmt.Fprintf(&envLines, "\n  %-24s %s", variable.Key, variable.Comment)
	}

	return &cobra.Command{
		Use:     transport.Name + " [name]",
		Aliases: transport.Aliases,
		Short:   "Add a " + transport.Title + " microservice",
		Long: fmt.Sprintf(`Add a %s microservice under apps/.

%s

With no name the service is created as apps/%s. The generated main.go
reads its settings from .env, which is extended with any of these it does not
already define:
%s

Run it with:
  nika start --watch -a <name>`,
			transport.Title, transport.Summary, transport.DefaultAppName(), envLines.String()),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return internal.RunMicroserviceAdd(&internal.MicroAddConfig{
				Transport: transport.Name,
				AppName:   name,
			})
		},
	}
}

func init() {
	microserviceInitCmd.Flags().StringVarP(&microInitName, "name", "n", "api",
		"Directory the current application moves into, under apps/")

	microserviceCmd.AddCommand(microserviceInitCmd)
	microserviceCmd.AddCommand(microserviceListCmd)
	for _, transport := range internal.MicroTransports() {
		microserviceCmd.AddCommand(newMicroTransportCmd(transport))
	}
	rootCmd.AddCommand(microserviceCmd)
}
