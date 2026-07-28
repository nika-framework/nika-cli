package aiagent

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/nika-framework/nika-cli/internal"
)

// The Commands tab.
//
// Not everything wants an LLM in the loop. "Generate a user module with these
// three fields" is a form, and running it as a form is faster, free, and
// deterministic. The chat tab is for the things a form cannot express.

// FieldKind tells the UI which control to render.
type FieldKind string

const (
	FieldText   FieldKind = "text"
	FieldSelect FieldKind = "select"
	FieldApp    FieldKind = "app"    // populated with the workspace's apps
	FieldFields FieldKind = "fields" // the repeating model-field editor
	FieldCheck  FieldKind = "check"
)

// CommandField is one input on a command form.
type CommandField struct {
	Name        string    `json:"name"`
	Label       string    `json:"label"`
	Kind        FieldKind `json:"kind"`
	Placeholder string    `json:"placeholder,omitempty"`
	Help        string    `json:"help,omitempty"`
	Options     []string  `json:"options,omitempty"`
	Default     string    `json:"default,omitempty"`
	Required    bool      `json:"required,omitempty"`
}

// Command is one runnable CLI action exposed in the UI.
type Command struct {
	ID          string         `json:"id"`
	Group       string         `json:"group"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	Preview     string         `json:"preview"`
	Fields      []CommandField `json:"fields"`
	// Mutates marks commands that change the project or the database.
	Mutates bool `json:"mutates"`
}

var databaseOptions = []string{"sqlite", "postgres", "mysql", "mongodb"}

// Commands returns the catalogue shown in the Commands tab.
func Commands() []Command {
	return []Command{
		{
			ID: "generate.res", Group: "Generate", Icon: "📦", Mutates: true,
			Title:       "Resource (full module)",
			Description: "Model, repository, DTOs, service, controller, response mapper, and module registration.",
			Preview:     "nika g res <module> -d <database> -a <app>",
			Fields: []CommandField{
				{Name: "module", Label: "Module name", Kind: FieldText, Placeholder: "product", Required: true, Help: "Lowercase, letters/digits/underscore."},
				{Name: "database", Label: "Database", Kind: FieldSelect, Options: databaseOptions, Default: "sqlite", Required: true},
				{Name: "app", Label: "App", Kind: FieldApp},
				{Name: "fields", Label: "Model fields", Kind: FieldFields, Help: "ID, CreatedAt and UpdatedAt are added automatically."},
			},
		},
		{
			ID: "generate.controller", Group: "Generate", Icon: "🎯", Mutates: true,
			Title: "Controller", Description: "Controller base plus the five CRUD handlers.",
			Preview: "nika g controller <module> -a <app>",
			Fields: []CommandField{
				{Name: "module", Label: "Module name", Kind: FieldText, Placeholder: "product", Required: true},
				{Name: "app", Label: "App", Kind: FieldApp},
			},
		},
		{
			ID: "generate.service", Group: "Generate", Icon: "⚙️", Mutates: true,
			Title: "Service", Description: "Service base plus one file per CRUD method.",
			Preview: "nika g service <module> -a <app>",
			Fields: []CommandField{
				{Name: "module", Label: "Module name", Kind: FieldText, Placeholder: "product", Required: true},
				{Name: "app", Label: "App", Kind: FieldApp},
			},
		},
		{
			ID: "generate.dto", Group: "Generate", Icon: "📥", Mutates: true,
			Title: "DTOs", Description: "Create, update, find-one, and list DTOs.",
			Preview: "nika g dto <module> -a <app>",
			Fields: []CommandField{
				{Name: "module", Label: "Module name", Kind: FieldText, Placeholder: "product", Required: true},
				{Name: "app", Label: "App", Kind: FieldApp},
			},
		},
		{
			ID: "generate.response", Group: "Generate", Icon: "📤", Mutates: true,
			Title: "Response + mapper", Description: "Response structs and the model-to-response mapper.",
			Preview: "nika g response <module> -a <app>",
			Fields: []CommandField{
				{Name: "module", Label: "Module name", Kind: FieldText, Placeholder: "product", Required: true},
				{Name: "app", Label: "App", Kind: FieldApp},
			},
		},
		{
			ID: "generate.migration", Group: "Database", Icon: "🗃️", Mutates: true,
			Title: "Migration", Description: "A new migration under internal/database/migrations.",
			Preview: "nika g migration <name> -d <database>",
			Fields: []CommandField{
				{Name: "name", Label: "Migration name", Kind: FieldText, Placeholder: "create_users", Required: true},
				{Name: "database", Label: "Database", Kind: FieldSelect, Options: databaseOptions, Default: "sqlite", Required: true},
				{Name: "format", Label: "Format", Kind: FieldSelect, Options: []string{"go", "sql"}, Default: "go"},
				{Name: "model", Label: "Model file", Kind: FieldText, Placeholder: "src/user/schema/user.model.go", Help: "Optional — generates real DDL from the struct's db tags instead of a stub."},
			},
		},
		{
			ID: "generate.seed", Group: "Database", Icon: "🌱", Mutates: true,
			Title: "Seed", Description: "A new seed under internal/database/seeds.",
			Preview: "nika g seed <name> -d <database>",
			Fields: []CommandField{
				{Name: "name", Label: "Seed name", Kind: FieldText, Placeholder: "initial_admins", Required: true},
				{Name: "database", Label: "Database", Kind: FieldSelect, Options: databaseOptions, Default: "sqlite", Required: true},
				{Name: "model", Label: "Model file", Kind: FieldText, Placeholder: "src/user/schema/user.model.go", Help: "Optional — builds a sample row from the struct."},
			},
		},
		{
			ID: "migrate.up", Group: "Database", Icon: "⬆️", Mutates: true,
			Title: "Run migrations", Description: "Apply every pending migration, or the next N.",
			Preview: "nika migrate up [n]",
			Fields: []CommandField{
				{Name: "count", Label: "How many", Kind: FieldText, Placeholder: "all"},
			},
		},
		{
			ID: "migrate.down", Group: "Database", Icon: "⬇️", Mutates: true,
			Title: "Roll back", Description: "Roll back the most recent migrations.",
			Preview: "nika migrate down [n]",
			Fields: []CommandField{
				{Name: "count", Label: "How many", Kind: FieldText, Default: "1"},
			},
		},
		{
			ID: "migrate.status", Group: "Database", Icon: "📋",
			Title: "Migration status", Description: "Applied versus pending migrations.",
			Preview: "nika migrate status",
		},
		{
			ID: "seed.run", Group: "Database", Icon: "▶️", Mutates: true,
			Title: "Run seeds", Description: "Run pending seeds, or only the named ones.",
			Preview: "nika seed run [names...]",
			Fields: []CommandField{
				{Name: "names", Label: "Seed names", Kind: FieldText, Placeholder: "leave empty for all"},
			},
		},
		{
			ID: "seed.status", Group: "Database", Icon: "📋",
			Title: "Seed status", Description: "Applied versus pending seeds.",
			Preview: "nika seed status",
		},
		{
			ID: "app.list", Group: "Workspace", Icon: "🧩",
			Title: "List apps", Description: "Every app, its source folder, run command, and modules.",
			Preview: "nika app list",
		},
		{
			ID: "app.use", Group: "Workspace", Icon: "📌", Mutates: true,
			Title: "Set default app", Description: "Choose which app `nika start` runs.",
			Preview: "nika app use <name>",
			Fields: []CommandField{
				{Name: "app", Label: "App", Kind: FieldApp, Required: true},
			},
		},
		{
			ID: "app.sync", Group: "Workspace", Icon: "🔄", Mutates: true,
			Title: "Sync workspace", Description: "Rewrite .nika.toml to match the apps on disk.",
			Preview: "nika app sync",
		},
		{
			ID: "swagger.init", Group: "Tools", Icon: "📘", Mutates: true,
			Title: "Generate Swagger docs", Description: "Run swag init over the project.",
			Preview: "swag init --dir <dir> --output <output>",
			Fields: []CommandField{
				{Name: "dir", Label: "Directory", Kind: FieldText, Default: "./", Help: "Where main.go lives, e.g. ./apps/api"},
				{Name: "output", Label: "Output", Kind: FieldText, Default: "./docs"},
			},
		},
		{
			ID: "go.build", Group: "Tools", Icon: "🔨",
			Title: "Build", Description: "Compile everything and report errors.",
			Preview: "go build ./...",
		},
		{
			ID: "go.test", Group: "Tools", Icon: "🧪",
			Title: "Test", Description: "Run the project's tests.",
			Preview: "go test ./...",
		},
	}
}

// CommandInput is a form submission from the UI.
type CommandInput struct {
	ID     string            `json:"id"`
	Values map[string]string `json:"values"`
	Fields []struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Required bool   `json:"required"`
	} `json:"fields"`
}

func (c CommandInput) value(name string) string {
	return strings.TrimSpace(c.Values[name])
}

// RunCommand executes a catalogue entry in dir and returns its output.
//
// Generators are called in-process so they behave exactly as the CLI does;
// the shell-outs are only for tools the CLI itself shells out to.
func RunCommand(dir string, input CommandInput, readOnly bool) (string, error) {
	command, ok := findCommand(input.ID)
	if !ok {
		return "", fmt.Errorf("unknown command %q", input.ID)
	}
	if command.Mutates && readOnly {
		return "", fmt.Errorf("%s changes the project, and this session is read-only", command.Title)
	}

	restore, err := chdir(dir)
	if err != nil {
		return "", err
	}
	defer restore()

	switch input.ID {
	case "generate.res":
		return runGenerateResource(input)
	case "generate.controller":
		return runGenerateLayer(internal.GenController, input)
	case "generate.service":
		return runGenerateLayer(internal.GenService, input)
	case "generate.dto":
		return runGenerateLayer(internal.GenDTO, input)
	case "generate.response":
		return runGenerateLayer(internal.GenResponse, input)
	case "generate.migration":
		return runGenerateMigration(input)
	case "generate.seed":
		return runGenerateSeed(input)
	case "migrate.up", "migrate.down":
		return captureCLI(func() error {
			sub := strings.TrimPrefix(input.ID, "migrate.")
			if count := input.value("count"); count != "" && count != "all" {
				return internal.RunMigrate(sub, count)
			}
			return internal.RunMigrate(sub)
		})
	case "migrate.status":
		return captureCLI(func() error { return internal.RunMigrate("status") })
	case "seed.run":
		return captureCLI(func() error {
			if names := strings.Fields(input.value("names")); len(names) > 0 {
				return internal.RunSeed("run", names...)
			}
			return internal.RunSeed("run")
		})
	case "seed.status":
		return captureCLI(func() error { return internal.RunSeed("status") })
	case "app.list":
		return runAppList()
	case "app.use":
		return runAppUse(input.value("app"))
	case "app.sync":
		return runAppSync()
	case "swagger.init":
		return runShell("swag", "init", "--dir", orDefault(input.value("dir"), "./"), "--output", orDefault(input.value("output"), "./docs"))
	case "go.build":
		return runShell("go", "build", "./...")
	case "go.test":
		return runShell("go", "test", "./...")
	}
	return "", fmt.Errorf("command %q has no runner", input.ID)
}

func findCommand(id string) (Command, bool) {
	for _, command := range Commands() {
		if command.ID == id {
			return command, true
		}
	}
	return Command{}, false
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func runGenerateResource(input CommandInput) (string, error) {
	module := input.value("module")
	if module == "" {
		return "", fmt.Errorf("module name is required")
	}
	database := internal.ParseDatabaseType(input.value("database"))
	if database == "" {
		return "", fmt.Errorf("choose a database")
	}
	if len(input.Fields) == 0 {
		return "", fmt.Errorf("add at least one model field")
	}

	fields := make([]internal.Field, 0, len(input.Fields))
	for _, field := range input.Fields {
		if strings.TrimSpace(field.Name) == "" {
			continue
		}
		built, err := internal.NewField(field.Name, field.Type, field.Required, database)
		if err != nil {
			return "", err
		}
		fields = append(fields, built)
	}
	if len(fields) == 0 {
		return "", fmt.Errorf("add at least one model field")
	}

	return captureCLI(func() error {
		return internal.RunGenerate(&internal.GenerateConfig{
			Type:     internal.GenResource,
			Module:   module,
			Database: string(database),
			App:      input.value("app"),
			Fields:   fields,
		})
	})
}

func runGenerateLayer(kind internal.GenerateType, input CommandInput) (string, error) {
	module := input.value("module")
	if module == "" {
		return "", fmt.Errorf("module name is required")
	}
	return captureCLI(func() error {
		return internal.RunGenerate(&internal.GenerateConfig{
			Type:   kind,
			Module: module,
			App:    input.value("app"),
		})
	})
}

func runGenerateMigration(input CommandInput) (string, error) {
	database := internal.ParseDatabaseType(input.value("database"))
	if database == "" {
		return "", fmt.Errorf("choose a database")
	}
	paths, err := internal.GenerateMigration(&internal.MigrationConfig{
		Name:     input.value("name"),
		Database: database,
		Format:   orDefault(input.value("format"), "go"),
		Model:    input.value("model"),
	})
	if err != nil {
		return "", err
	}
	return "created:\n  " + strings.Join(paths, "\n  "), nil
}

func runGenerateSeed(input CommandInput) (string, error) {
	database := internal.ParseDatabaseType(input.value("database"))
	if database == "" {
		return "", fmt.Errorf("choose a database")
	}
	path, err := internal.GenerateSeed(&internal.SeedConfig{
		Name:     input.value("name"),
		Database: database,
		Model:    input.value("model"),
	})
	if err != nil {
		return "", err
	}
	return "created " + path, nil
}

func runAppList() (string, error) {
	workspace, err := internal.LoadWorkspace()
	if err != nil {
		return "", err
	}
	var out strings.Builder
	if workspace.Microservice {
		fmt.Fprintf(&out, "Microservice workspace — module %s\n\n", workspace.ModulePath)
	} else {
		fmt.Fprintf(&out, "Single application — module %s\n\n", workspace.ModulePath)
	}
	for _, app := range workspace.Apps {
		marker := " "
		if app.Name == workspace.DefaultApp {
			marker = "*"
		}
		fmt.Fprintf(&out, " %s %-16s src: %-24s run: %s\n", marker, app.Name, app.SrcDir, app.RunCommand())
		if modules := workspace.Modules(app); len(modules) > 0 {
			fmt.Fprintf(&out, "   %-16s modules: %s\n", "", strings.Join(modules, ", "))
		}
	}
	return out.String(), nil
}

func runAppUse(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("choose an app")
	}
	workspace, err := internal.LoadWorkspace()
	if err != nil {
		return "", err
	}
	if err := workspace.SetDefaultApp(name); err != nil {
		return "", err
	}
	target := workspace.Find(workspace.DefaultApp)
	return fmt.Sprintf("Default app is now %q (%s)", workspace.DefaultApp, target.RunCommand()), nil
}

func runAppSync() (string, error) {
	workspace, err := internal.LoadWorkspace()
	if err != nil {
		return "", err
	}
	if err := workspace.Sync(); err != nil {
		return "", err
	}
	return fmt.Sprintf("Updated .nika.toml: %d app(s) — %s\nDefault app: %s → %s",
		len(workspace.Apps), strings.Join(workspace.AppNames(), ", "),
		workspace.Config.Workspace.DefaultApp, workspace.Config.Build.Cmd), nil
}

func runShell(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			return "", err
		}
		// Return the output as the result: a failing build is information, and
		// hiding it behind a bare exit status helps nobody.
		return fmt.Sprintf("%s\n\n(exited with %v)", text, err), nil
	}
	if text == "" {
		return "Done — no output.", nil
	}
	return text, nil
}

// captureCLI runs a generator that prints to stdout and returns what it
// printed, so the browser sees the same progress the terminal would.
func captureCLI(run func() error) (string, error) {
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = writer

	done := make(chan string, 1)
	go func() {
		var buffer bytes.Buffer
		_, _ = buffer.ReadFrom(reader)
		done <- buffer.String()
	}()

	runErr := run()

	os.Stdout = original
	_ = writer.Close()
	output := strings.TrimSpace(<-done)
	_ = reader.Close()

	if runErr != nil {
		if output != "" {
			return "", fmt.Errorf("%s\n\n%w", output, runErr)
		}
		return "", runErr
	}
	return output, nil
}
