package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/nika-framework/nika-cli/common"
	"github.com/nika-framework/nika-cli/internal/nikaconf"
	"github.com/nika-framework/nika-cli/templates"
)

// The microservice scaffolder.
//
// A Nika project starts as one application with its modules in src/. Growing a
// second process out of that means two things the user should not have to do by
// hand: moving src/ under apps/<name>/ and rewriting every import that pointed
// at it, and writing the twenty lines of transport wiring that differ per
// broker. `nika microservice init` does the first, `nika microservice <transport>`
// the second.

// MicroEnv is one environment variable a transport needs.
type MicroEnv struct {
	Key     string
	Value   string
	Comment string
}

// MicroTransport describes one supported transport: which framework package
// implements it, which template renders its main.go, and what it needs from the
// environment.
type MicroTransport struct {
	// Name is the canonical CLI name ("rabbit") and the sub-command.
	Name string
	// Aliases are the other spellings accepted on the command line.
	Aliases []string
	// Tag is the value of the `transport:"..."` struct tag, which must match
	// what the framework's Listener reports as its Name().
	Tag string
	// Pkg is the framework transport package ("rabbitmq"), and also the
	// template directory under templates/micro.
	Pkg string
	// Title is the human name shown in listings.
	Title string
	// Summary is the one-line trade-off, printed after scaffolding so the
	// choice is visible at the moment it is made.
	Summary string
	// env returns the variables to append to .env, given the app name.
	env func(app string) []MicroEnv
}

// DefaultAppName is the directory this transport gets when the user names none:
// `nika microservice grpc` creates apps/grpc-micro.
func (t MicroTransport) DefaultAppName() string { return t.Name + "-micro" }

// Env lists the environment variables the generated service reads.
func (t MicroTransport) Env(app string) []MicroEnv { return t.env(app) }

// microTransports is the catalogue, in the order `nika microservice list`
// prints it: brokers first, then the two direct transports.
var microTransports = []MicroTransport{
	{
		Name: "kafka", Tag: "kafka", Pkg: "kafkamq", Title: "Apache Kafka",
		Summary: "Ordered, replayable event log. At-least-once; a consumer group is required.",
		env: func(app string) []MicroEnv {
			return []MicroEnv{
				{"KAFKA_BROKERS", "localhost:9092", "Comma-separated bootstrap brokers"},
				{"KAFKA_TOPIC", "nika", "One topic carries every message; the pattern is the key"},
				{"KAFKA_GROUP_ID", app, "Consumer group — without it every replica reads every partition"},
				{"KAFKA_CONCURRENCY", "1", "Above 1 gives up per-partition ordering"},
			}
		},
	},
	{
		Name: "nats", Tag: "nats", Pkg: "natsmq", Title: "NATS",
		Summary: "Lowest-friction broker with native request/reply. At-most-once on core NATS.",
		env: func(app string) []MicroEnv {
			return []MicroEnv{
				{"NATS_URL", "nats://localhost:4222", "Server URL, or a comma-separated cluster"},
				{"NATS_PREFIX", "nika", "Namespaces every subject; the NATS subject space is global"},
				{"NATS_NAME", app, "Connection name shown in `nats server report connections`"},
				{"NATS_QUEUE_GROUP", app, "Set = load-balanced across replicas; empty = broadcast"},
			}
		},
	},
	{
		Name: "rabbit", Aliases: []string{"rabbitmq", "amqp"}, Tag: "rabbitmq", Pkg: "rabbitmq", Title: "RabbitMQ",
		Summary: "Topic exchange, durable queues, at-least-once with publisher confirms.",
		env: func(app string) []MicroEnv {
			return []MicroEnv{
				{"RABBITMQ_URL", "amqp://guest:guest@localhost:5672/", "AMQP dial string"},
				{"RABBITMQ_EXCHANGE", "nika", "Topic exchange to publish to and bind against"},
				{"RABBITMQ_QUEUE", "nika." + app, "Must be distinct per service, or they compete for messages"},
				{"RABBITMQ_PREFETCH", "32", "Unacknowledged deliveries per consumer"},
			}
		},
	},
	{
		Name: "redis", Aliases: []string{"redismq"}, Tag: "redis", Pkg: "redismq", Title: "Redis pub/sub",
		Summary: "At-most-once and stores nothing — right for invalidation and presence, wrong for orders.",
		env: func(string) []MicroEnv {
			return []MicroEnv{
				{"REDIS_MQ_URL", "redis://localhost:6379", "Connection string (rediss:// for TLS)"},
				{"REDIS_MQ_PREFIX", "nika", "Namespaces every channel; pub/sub has no vhosts"},
			}
		},
	},
	{
		Name: "grpc", Aliases: []string{"grpcmq"}, Tag: "grpc", Pkg: "grpcmq", Title: "gRPC",
		Summary: "Synchronous RPC, no broker and no store-and-forward. No protoc step.",
		env: func(string) []MicroEnv {
			return []MicroEnv{
				{"GRPC_ADDR", ":50051", "Listen address for this service"},
				{"GRPC_TARGET", "", "Address this service calls out to; leave empty if it only serves"},
				{"GRPC_INSECURE", "true", "Plaintext gRPC. Set false and supply TLS before this leaves localhost"},
			}
		},
	},
	{
		Name: "tcp", Aliases: []string{"tcpmq"}, Tag: "tcp", Pkg: "tcpmq", Title: "Raw TCP",
		Summary: "No broker at all. Nothing to run in tests; nothing to absorb a restart either.",
		env: func(string) []MicroEnv {
			return []MicroEnv{
				{"TCP_ADDR", ":4000", "Bind address for this service"},
				{"TCP_DIAL_ADDR", "", "Peer address for the client half; defaults to TCP_ADDR"},
			}
		},
	},
}

// MicroTransports returns the catalogue.
func MicroTransports() []MicroTransport { return microTransports }

// FindMicroTransport resolves a name or alias, case-insensitively.
func FindMicroTransport(name string) (MicroTransport, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, transport := range microTransports {
		if transport.Name == name {
			return transport, true
		}
		for _, alias := range transport.Aliases {
			if alias == name {
				return transport, true
			}
		}
	}
	return MicroTransport{}, false
}

// MicroTransportNames lists every accepted spelling, for error messages.
func MicroTransportNames() []string {
	names := make([]string, 0, len(microTransports))
	for _, transport := range microTransports {
		names = append(names, transport.Name)
	}
	return names
}

// ── init: single app → apps/<name> ──────────────────────────────────

// MicroInitConfig parameterises `nika microservice init`.
type MicroInitConfig struct {
	// AppName is the directory the current application moves into. Defaults
	// to "api".
	AppName string
}

// RunMicroserviceInit converts a single-application project into a
// microservice workspace.
//
// It moves src/ and main.go under apps/<name>/, rewrites every import that
// pointed at the old locations, and records the new layout in .nika.toml. The
// import rewrite is the part that cannot be left to the user: a project with
// twenty modules has hundreds of `<module>/src/...` imports and missing one
// leaves a build that fails somewhere unrelated.
func RunMicroserviceInit(config *MicroInitConfig) error {
	appName := strings.TrimSpace(config.AppName)
	if appName == "" {
		appName = "api"
	}
	if err := validateAppName(appName); err != nil {
		return err
	}

	sp := common.NewSpinner()
	common.Section("Workspace Check")
	sp.Start("Inspecting the project layout...")

	workspace, err := LoadWorkspace()
	if err != nil {
		sp.Fail(err.Error())
		return err
	}
	if workspace.Microservice {
		sp.Fail("This project is already a microservice workspace")
		return fmt.Errorf("already a microservice workspace — apps: %s", strings.Join(workspace.AppNames(), ", "))
	}

	root := workspace.Root
	appsDir, srcName := microLayout(workspace)

	srcPath := filepath.Join(root, srcName)
	if !dirExists(srcPath) {
		sp.Fail(fmt.Sprintf("No %s/ directory to move", srcName))
		return fmt.Errorf("no %s/ directory in %s — nothing to convert", srcName, root)
	}
	appDir := filepath.Join(root, appsDir, appName)
	if _, err := os.Stat(appDir); err == nil {
		sp.Fail(fmt.Sprintf("%s already exists", path(appsDir, appName)))
		return fmt.Errorf("%s already exists", path(appsDir, appName))
	}
	sp.Step(fmt.Sprintf("Single application, module %s", workspace.ModulePath), "")

	// Record the layout before moving anything. discoverApps reads apps_dir
	// from the file, so a config still saying apps_dir = "src" would make the
	// reload below find no apps at all and write mode = "single" back over the
	// conversion that just happened.
	layout := workspace.Config
	layout.Workspace.Mode = "microservice"
	layout.Workspace.AppsDir = appsDir
	layout.Workspace.SrcDir = srcName
	layout.Workspace.DefaultApp = appName
	if err := nikaconf.SaveFrom(root, layout); err != nil {
		sp.Fail(err.Error())
		return fmt.Errorf("updating %s: %w", nikaconf.FileName, err)
	}

	// ── Move ──────────────────────────────────────────────────────
	common.Section("Restructuring")
	sp.Start(fmt.Sprintf("Creating %s...", path(appsDir, appName)))
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		sp.Fail(err.Error())
		return err
	}
	sp.Step(fmt.Sprintf("%s created", path(appsDir, appName)), fmt.Sprintf("Moving %s/...", srcName))

	// moved records old → new import fragments for the rewrite below.
	moved := map[string]string{}

	if err := os.Rename(srcPath, filepath.Join(appDir, srcName)); err != nil {
		sp.Fail(err.Error())
		return fmt.Errorf("moving %s: %w", srcName, err)
	}
	moved[srcName] = path(appsDir, appName, srcName)
	sp.Step(fmt.Sprintf("%s → %s", srcName, path(appsDir, appName, srcName)), "Moving main.go...")

	mainGo := filepath.Join(root, "main.go")
	if fileExists(mainGo) {
		if err := os.Rename(mainGo, filepath.Join(appDir, "main.go")); err != nil {
			sp.Fail(err.Error())
			return fmt.Errorf("moving main.go: %w", err)
		}
		sp.Step(fmt.Sprintf("main.go → %s", path(appsDir, appName, "main.go")), "")
	} else {
		sp.Step("No main.go at the root — skipped", "")
	}

	// Swagger output belongs to the app whose annotations produced it, and
	// main.go imports it with a blank identifier.
	docsDir := filepath.Join(root, "docs")
	if fileExists(filepath.Join(docsDir, "docs.go")) {
		sp.Start("Moving docs/...")
		if err := os.Rename(docsDir, filepath.Join(appDir, "docs")); err != nil {
			sp.Fail(err.Error())
			return fmt.Errorf("moving docs: %w", err)
		}
		moved["docs"] = path(appsDir, appName, "docs")
		sp.Step(fmt.Sprintf("docs → %s", path(appsDir, appName, "docs")), "")
	}

	// ── Rewrite imports ───────────────────────────────────────────
	common.Section("Rewriting Imports")
	sp.Start("Scanning .go files...")
	rewritten, err := rewriteModuleImports(root, workspace.ModulePath, moved)
	if err != nil {
		sp.Fail(err.Error())
		return err
	}
	sp.Step(fmt.Sprintf("Updated %d file(s)", len(rewritten)), "")

	// ── Config ────────────────────────────────────────────────────
	common.Section("Updating .nika.toml")
	sp.Start("Recording the new layout...")
	reloaded, err := LoadWorkspaceAt(root)
	if err != nil {
		sp.Fail(err.Error())
		return err
	}
	reloaded.Config.Workspace.DefaultApp = appName
	reloaded.DefaultApp = appName
	if err := reloaded.Sync(); err != nil {
		sp.Fail(err.Error())
		return err
	}
	sp.Stop(fmt.Sprintf("mode = microservice, default_app = %q", appName))

	// ── Done ──────────────────────────────────────────────────────
	common.Section("Done!")
	fmt.Println()
	fmt.Printf("  🧩 Converted to a microservice workspace.\n")
	fmt.Println()
	fmt.Println("  Layout:")
	fmt.Printf("    %s/%s/\n", appsDir, appName)
	if fileExists(filepath.Join(appDir, "main.go")) {
		fmt.Printf("      main.go\n")
	}
	fmt.Printf("      %s/            (your modules, unchanged)\n", srcName)
	if len(rewritten) > 0 {
		fmt.Println()
		fmt.Printf("  Imports rewritten in %d file(s):\n", len(rewritten))
		for _, file := range firstN(rewritten, 8) {
			fmt.Printf("    %s\n", file)
		}
		if len(rewritten) > 8 {
			fmt.Printf("    … and %d more\n", len(rewritten)-8)
		}
	}
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("    go build ./...")
	fmt.Printf("    nika start --watch -a %s\n", appName)
	fmt.Printf("    nika microservice grpc          # add a second service\n")
	fmt.Println()
	return nil
}

// ── add: apps/<name> for one transport ──────────────────────────────

// MicroAddConfig parameterises `nika microservice <transport> [name]`.
type MicroAddConfig struct {
	// Transport is the CLI name or alias ("grpc", "rabbitmq", …).
	Transport string
	// AppName is the directory to create. Empty means <transport>-micro.
	AppName string
}

// microTemplateData is what the templates under templates/micro see.
type microTemplateData struct {
	// ModulePath is the Go module path from go.mod.
	ModulePath string
	// AppName is the service directory name, e.g. "grpc-micro".
	AppName string
	// SrcImport is the import-path fragment of the service's src folder.
	SrcImport string
	// Transport is the `transport:"..."` tag value.
	Transport string
	// Subject prefixes the sample handlers' patterns.
	Subject string
	// EnvPath is the .env location relative to the working directory the
	// process is started from — the project root, for every `nika start`.
	EnvPath string
}

// RunMicroserviceAdd scaffolds apps/<name> for one transport.
func RunMicroserviceAdd(config *MicroAddConfig) error {
	transport, ok := FindMicroTransport(config.Transport)
	if !ok {
		return fmt.Errorf("unknown transport %q — available: %s",
			config.Transport, strings.Join(MicroTransportNames(), ", "))
	}
	appName := strings.TrimSpace(config.AppName)
	if appName == "" {
		appName = transport.DefaultAppName()
	}
	if err := validateAppName(appName); err != nil {
		return err
	}

	sp := common.NewSpinner()
	common.Section("Workspace Check")
	sp.Start("Inspecting the project layout...")

	workspace, err := LoadWorkspace()
	if err != nil {
		sp.Fail(err.Error())
		return err
	}
	if !workspace.Microservice {
		sp.Fail("This project is still a single application")
		return fmt.Errorf("run `nika microservice init` first — it moves src/ under apps/ so a second service can exist alongside it")
	}
	if existing := workspace.Find(appName); existing != nil && strings.EqualFold(existing.Name, appName) {
		sp.Fail(fmt.Sprintf("App %q already exists", appName))
		return fmt.Errorf("app %q already exists at %s", appName, existing.Dir)
	}

	root := workspace.Root
	appsDir, srcName := microLayout(workspace)
	appDir := filepath.Join(root, appsDir, appName)
	if _, err := os.Stat(appDir); err == nil {
		sp.Fail(fmt.Sprintf("%s already exists", path(appsDir, appName)))
		return fmt.Errorf("%s already exists", path(appsDir, appName))
	}
	sp.Step(fmt.Sprintf("Microservice workspace, module %s", workspace.ModulePath), "")

	data := &microTemplateData{
		ModulePath: workspace.ModulePath,
		AppName:    appName,
		SrcImport:  path(appsDir, appName, srcName),
		Transport:  transport.Tag,
		Subject:    messageSubject(appName),
		EnvPath:    ".env",
	}

	// ── Scaffold ──────────────────────────────────────────────────
	common.Section(fmt.Sprintf("Generating %s Service", transport.Title))
	files := []struct {
		tpl   string
		out   string
		label string
	}{
		{"micro/" + transport.Pkg + "/main.go.tpl", filepath.Join(appDir, "main.go"), "main.go"},
		{"micro/app.module.go.tpl", filepath.Join(appDir, srcName, "app.module.go"), "app.module.go"},
		{"micro/app.controller.go.tpl", filepath.Join(appDir, srcName, "app.controller.go"), "app.controller.go"},
	}
	for _, file := range files {
		sp.Start(fmt.Sprintf("Rendering %s...", file.label))
		content, err := templates.Read(file.tpl)
		if err != nil {
			sp.Fail(err.Error())
			return fmt.Errorf("template %s not found in binary: %w", file.tpl, err)
		}
		rendered, err := common.RenderString(file.tpl, content, data)
		if err != nil {
			sp.Fail(err.Error())
			return err
		}
		if err := common.WriteRendered(file.out, rendered); err != nil {
			sp.Fail(err.Error())
			return err
		}
		sp.Step(fmt.Sprintf("✔ %s created", file.label), "")
	}

	// ── Environment ───────────────────────────────────────────────
	common.Section("Environment")
	env := transport.Env(appName)
	added, err := appendEnvVars(root, appName, transport, env)
	if err != nil {
		return err
	}
	if len(added) == 0 {
		fmt.Printf("  ✔ All %d variable(s) already present\n", len(env))
	} else {
		for _, key := range added {
			fmt.Printf("  ✔ %s\n", key)
		}
	}

	// ── Config ────────────────────────────────────────────────────
	common.Section("Updating .nika.toml")
	sp.Start("Registering the new app...")
	reloaded, err := LoadWorkspaceAt(root)
	if err != nil {
		sp.Fail(err.Error())
		return err
	}
	if err := reloaded.Sync(); err != nil {
		sp.Fail(err.Error())
		return err
	}
	target := reloaded.Find(appName)
	if target == nil {
		sp.Fail("the new app was not detected on disk")
		return fmt.Errorf("generated %s but LoadWorkspace did not detect it", path(appsDir, appName))
	}
	sp.Stop(fmt.Sprintf("[apps.%s] cmd = %q", appName, target.RunCommand()))

	// ── Done ──────────────────────────────────────────────────────
	common.Section("Done!")
	fmt.Println()
	fmt.Printf("  🧩 %s service %q generated.\n", transport.Title, appName)
	fmt.Printf("     %s\n", transport.Summary)
	fmt.Println()
	fmt.Println("  Files created:")
	fmt.Printf("    %s\n", path(appsDir, appName, "main.go"))
	fmt.Printf("    %s\n", path(appsDir, appName, srcName, "app.module.go"))
	fmt.Printf("    %s\n", path(appsDir, appName, srcName, "app.controller.go"))
	fmt.Println()
	fmt.Println("  Sample patterns:")
	fmt.Printf("    %s_ping    %s_echo\n", data.Subject, data.Subject)
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("    go mod tidy")
	fmt.Printf("    nika g res <module> -a %s     # generate into this service\n", appName)
	fmt.Printf("    nika start --watch -a %s\n", appName)
	fmt.Println("    nika start --watch -a           # every service at once")
	fmt.Println()
	return nil
}

// ── helpers ─────────────────────────────────────────────────────────

// microLayout resolves the apps/ and src/ folder names to use.
//
// Disk beats the config file. A project whose .nika.toml says apps_dir = "src"
// — which older versions of `nika app sync` wrote for single-app projects — is
// otherwise told to create apps/api inside src/, and the move becomes a rename
// of a directory into itself.
func microLayout(workspace *Workspace) (appsDir, srcName string) {
	srcName = strings.TrimSpace(workspace.Config.Workspace.SrcDir)
	if srcName == "" {
		srcName = "src"
	}
	if workspace.Microservice && len(workspace.Apps) > 0 && workspace.Apps[0].Dir != "" {
		if parent, _, found := strings.Cut(workspace.Apps[0].Dir, "/"); found {
			return parent, srcName
		}
	}
	appsDir = strings.TrimSpace(workspace.Config.Workspace.AppsDir)
	if appsDir == "" || appsDir == srcName {
		appsDir = "apps"
	}
	return appsDir, srcName
}

// subjectUnsafe is every character that cannot appear in a pattern usable on
// all six transports at once.
var subjectUnsafe = regexp.MustCompile(`[^a-z0-9_]+`)

// messageSubject turns an app name into a pattern prefix.
//
// '_' is the only separator every transport accepts: RabbitMQ rejects '.'
// outright because it is AMQP's topic word separator, and NATS treats it as a
// token boundary its wildcards then behave differently around. A hyphen is
// legal but reads oddly next to '_ping', so both collapse to '_'.
func messageSubject(appName string) string {
	subject := subjectUnsafe.ReplaceAllString(strings.ToLower(appName), "_")
	return strings.Trim(subject, "_")
}

// appNamePattern is what is safe as both a directory name and a path segment
// in a Go import path.
var appNamePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)

func validateAppName(name string) error {
	if !appNamePattern.MatchString(name) {
		return fmt.Errorf("invalid app name %q — use lowercase letters, digits, '-', '_' or '.', starting with a letter", name)
	}
	if name == "apps" || name == "internal" || name == "cmd" || name == "vendor" {
		return fmt.Errorf("app name %q collides with a reserved project directory", name)
	}
	return nil
}

// rewriteModuleImports rewrites `<module>/<old>` import paths to
// `<module>/<new>` across every .go file under root, and returns the paths it
// changed, relative to root.
//
// The trailing boundary matters: replacing the bare prefix `nikaapp/src` would
// also rewrite `nikaapp/srcutil`, so only `…/src"` and `…/src/` are matched.
func rewriteModuleImports(root, modulePath string, moved map[string]string) ([]string, error) {
	if len(moved) == 0 {
		return nil, nil
	}
	type replacement struct{ old, new string }
	var pairs []replacement
	for from, to := range moved {
		pairs = append(pairs,
			replacement{fmt.Sprintf("%q", modulePath+"/"+from), fmt.Sprintf("%q", modulePath+"/"+to)},
			replacement{`"` + modulePath + "/" + from + "/", `"` + modulePath + "/" + to + "/"},
		)
	}
	// Longest first, so a nested move cannot be half-applied by a shorter one.
	sort.Slice(pairs, func(i, j int) bool { return len(pairs[i].old) > len(pairs[j].old) })

	var changed []string
	skipDirs := map[string]bool{".git": true, "vendor": true, "node_modules": true, "tmp": true}

	err := filepath.WalkDir(root, func(p string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skipDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(p) != ".go" {
			return nil
		}
		source, err := common.ReadFile(p)
		if err != nil {
			return err
		}
		updated := source
		for _, pair := range pairs {
			updated = strings.ReplaceAll(updated, pair.old, pair.new)
		}
		if updated == source {
			return nil
		}
		if err := common.WriteFile(p, updated); err != nil {
			return err
		}
		relative, relErr := filepath.Rel(root, p)
		if relErr != nil {
			relative = p
		}
		changed = append(changed, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(changed)
	return changed, nil
}

// appendEnvVars adds the transport's variables to .env and .env.example,
// skipping keys either file already defines, and returns the keys it added.
//
// Skipping rather than overwriting is the important half: re-running the
// generator, or adding a second service on the same broker, must not reset a
// URL the user has already pointed at their own infrastructure.
func appendEnvVars(root, appName string, transport MicroTransport, env []MicroEnv) ([]string, error) {
	var added []string
	for _, name := range []string{".env", ".env.example"} {
		keys, err := appendEnvFile(filepath.Join(root, name), appName, transport, env)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			if !contains(added, key) {
				added = append(added, key)
			}
		}
	}
	return added, nil
}

func appendEnvFile(path, appName string, transport MicroTransport, env []MicroEnv) ([]string, error) {
	existing := ""
	if content, err := common.ReadFile(path); err == nil {
		existing = content
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var block strings.Builder
	var added []string
	for _, variable := range env {
		if envKeyDefined(existing, variable.Key) {
			continue
		}
		if variable.Comment != "" {
			fmt.Fprintf(&block, "# %s\n", variable.Comment)
		}
		fmt.Fprintf(&block, "%s=%s\n", variable.Key, variable.Value)
		added = append(added, variable.Key)
	}
	if block.Len() == 0 {
		return nil, nil
	}

	var out strings.Builder
	out.WriteString(existing)
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		out.WriteString("\n")
	}
	if existing != "" {
		out.WriteString("\n")
	}
	fmt.Fprintf(&out, "# ── %s (%s microservice) ──\n", appName, transport.Title)
	out.WriteString(block.String())

	if err := common.WriteFile(path, out.String()); err != nil {
		return nil, err
	}
	return added, nil
}

// envKeyDefined reports whether content already assigns key, ignoring
// commented-out lines.
func envKeyDefined(content, key string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if name, _, found := strings.Cut(trimmed, "="); found && strings.TrimSpace(name) == key {
			return true
		}
	}
	return false
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func firstN(list []string, n int) []string {
	if len(list) <= n {
		return list
	}
	return list[:n]
}
