package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nika-framework/nika-cli/common"
	"github.com/nika-framework/nika-cli/internal/nikaconf"
)

// AppTarget is one runnable application inside the project.
//
// In the classic single-app layout there is exactly one target whose Dir is ""
// and whose SrcDir is "src". In a microservice workspace there is one target
// per directory under apps/, and SrcDir becomes "apps/<name>/src". Every path
// the generator writes and every import path it renders is derived from this
// struct, which is what makes `nika g res user` land in the right service
// instead of always creating a top-level src/.
type AppTarget struct {
	// Name is the service name ("api"), or "app" for the single-app layout.
	Name string
	// Dir is the app root relative to the project root, in slash form.
	// Empty for the single-app layout.
	Dir string
	// SrcDir is where modules live, relative to the project root, slash form.
	SrcDir string
	// MainGo is the entry file relative to the project root, slash form.
	MainGo string
}

// SrcImport is the import-path fragment for SrcDir, e.g. "apps/api/src".
func (a AppTarget) SrcImport() string { return a.SrcDir }

// SrcPath is SrcDir in OS-native form, for filesystem calls.
func (a AppTarget) SrcPath() string { return filepath.FromSlash(a.SrcDir) }

// ModuleDir is the on-disk directory of one generated module.
func (a AppTarget) ModuleDir(module string) string {
	return filepath.Join(a.SrcPath(), module)
}

// AppModulePath is the app.module.go that must import new modules.
func (a AppTarget) AppModulePath() string {
	return filepath.Join(a.SrcPath(), "app.module.go")
}

// RunCommand is the `go run` invocation that starts this app.
func (a AppTarget) RunCommand() string {
	if a.MainGo == "" || a.MainGo == "main.go" {
		return "go run ."
	}
	return "go run ./" + a.MainGo
}

// Label is what the interactive picker shows.
func (a AppTarget) Label() string {
	if a.Dir == "" {
		return a.Name + "  (src/)"
	}
	return a.Name + "  (" + a.SrcDir + ")"
}

// Workspace is the resolved project layout.
type Workspace struct {
	// Root is the absolute project root (the directory holding go.mod).
	Root string
	// ModulePath is the Go module path from go.mod.
	ModulePath string
	// Microservice reports whether the project has an apps/ directory.
	Microservice bool
	// Apps is every runnable target, sorted by name.
	Apps []AppTarget
	// DefaultApp is the configured default, may be empty.
	DefaultApp string
	// Config is the parsed .nika.toml.
	Config nikaconf.Config
	// configExists records whether .nika.toml was on disk.
	configExists bool
}

// LoadWorkspace resolves the layout of the project in the current directory.
//
// The config file is authoritative when it lists apps, but disk always wins on
// existence: a service that was deleted is dropped, and one that was added by
// hand is picked up without the user editing .nika.toml first.
func LoadWorkspace() (*Workspace, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return LoadWorkspaceAt(root)
}

// LoadWorkspaceAt resolves the layout of the project rooted at dir.
func LoadWorkspaceAt(dir string) (*Workspace, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(abs, "go.mod")); err != nil {
		return nil, fmt.Errorf("not in a Go project (no go.mod in %s)", abs)
	}
	modulePath, err := resolveModulePathIn(abs)
	if err != nil {
		return nil, err
	}
	config, exists, err := nikaconf.LoadFrom(abs)
	if err != nil {
		return nil, err
	}
	for _, warning := range nikaconf.TakeWarnings() {
		fmt.Printf("  ⚠ %s\n", warning)
	}

	workspace := &Workspace{
		Root:         abs,
		ModulePath:   modulePath,
		DefaultApp:   strings.TrimSpace(config.Workspace.DefaultApp),
		Config:       config,
		configExists: exists,
	}
	workspace.Apps = discoverApps(abs, config)
	workspace.Microservice = len(workspace.Apps) > 0 && workspace.Apps[0].Dir != ""

	if len(workspace.Apps) == 0 {
		// Nothing detected: fall back to the classic layout so a brand-new
		// project still generates into src/ before that folder exists.
		srcDir := config.Workspace.SrcDir
		if srcDir == "" {
			srcDir = "src"
		}
		workspace.Apps = []AppTarget{{Name: "app", SrcDir: srcDir, MainGo: "main.go"}}
	}
	if workspace.DefaultApp != "" && workspace.Find(workspace.DefaultApp) == nil {
		workspace.DefaultApp = ""
	}
	return workspace, nil
}

// discoverApps walks apps/ (or whatever apps_dir names) and falls back to the
// single-app layout.
func discoverApps(root string, config nikaconf.Config) []AppTarget {
	appsDir := config.Workspace.AppsDir
	if appsDir == "" {
		appsDir = "apps"
	}
	srcName := config.Workspace.SrcDir
	if srcName == "" {
		srcName = "src"
	}

	// An explicit mode = "single" pins the classic layout even when apps/
	// happens to exist for unrelated reasons.
	if config.Workspace.Mode != "single" {
		entries, err := os.ReadDir(filepath.Join(root, appsDir))
		if err == nil {
			var apps []AppTarget
			for _, entry := range entries {
				if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
					continue
				}
				target, ok := inspectApp(root, appsDir, srcName, entry.Name())
				if ok {
					apps = append(apps, target)
				}
			}
			if len(apps) > 0 {
				sort.Slice(apps, func(i, j int) bool { return apps[i].Name < apps[j].Name })
				return apps
			}
		}
	}

	if dirExists(filepath.Join(root, srcName)) || fileExists(filepath.Join(root, "main.go")) {
		return []AppTarget{{Name: "app", SrcDir: srcName, MainGo: "main.go"}}
	}
	return nil
}

// inspectApp accepts a directory under apps/ as a service when it has either a
// main.go or a src/ folder — the two things that make it a Nika app rather
// than, say, a proto or docs folder that happens to sit alongside them.
func inspectApp(root, appsDir, srcName, name string) (AppTarget, bool) {
	dir := filepath.Join(root, appsDir, name)
	hasMain := fileExists(filepath.Join(dir, "main.go"))
	hasSrc := dirExists(filepath.Join(dir, srcName))
	if !hasMain && !hasSrc {
		return AppTarget{}, false
	}
	target := AppTarget{
		Name:   name,
		Dir:    path(appsDir, name),
		SrcDir: path(appsDir, name, srcName),
	}
	if hasMain {
		target.MainGo = path(appsDir, name, "main.go")
	}
	return target, true
}

// Find returns the app with the given name, or nil.
//
// Matching is deliberately forgiving: users type "grpc" for "micro-grpc" far
// more often than they type the full directory name.
func (w *Workspace) Find(name string) *AppTarget {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	lower := strings.ToLower(name)
	for i := range w.Apps {
		if strings.EqualFold(w.Apps[i].Name, name) {
			return &w.Apps[i]
		}
	}
	// Unique substring match ("grpc" → "micro-grpc").
	var match *AppTarget
	for i := range w.Apps {
		if strings.Contains(strings.ToLower(w.Apps[i].Name), lower) {
			if match != nil {
				return nil // ambiguous — make the caller be explicit
			}
			match = &w.Apps[i]
		}
	}
	return match
}

// SelectApp picks the app a command should operate on.
//
// Order of precedence: the --app flag, then NIKA_APP, then the only app if
// there is one, then an interactive prompt. The prompt is the point: in a
// microservice workspace "which service?" is a question the CLI cannot guess,
// and guessing wrong writes a whole module into the wrong process.
func (w *Workspace) SelectApp(requested string) (AppTarget, error) {
	if requested == "" {
		requested = strings.TrimSpace(os.Getenv("NIKA_APP"))
	}
	if requested != "" {
		target := w.Find(requested)
		if target == nil {
			return AppTarget{}, fmt.Errorf("unknown app %q — available: %s", requested, strings.Join(w.AppNames(), ", "))
		}
		return *target, nil
	}
	if len(w.Apps) == 1 {
		return w.Apps[0], nil
	}

	options := make([]string, len(w.Apps))
	for i, app := range w.Apps {
		options[i] = app.Label()
	}
	choice := common.SelectOption("Which microservice should this belong to?", options)
	for i, option := range options {
		if option == choice {
			return w.Apps[i], nil
		}
	}
	return w.Apps[0], nil
}

// FindModule locates which app already contains a generated module.
//
// It returns false when the module exists in more than one app, because
// "user" living in three services is exactly the case where the CLI must ask
// rather than pick.
func (w *Workspace) FindModule(module string) (AppTarget, bool) {
	var found AppTarget
	count := 0
	for _, app := range w.Apps {
		if dirExists(filepath.Join(w.Root, app.ModuleDir(module))) {
			found = app
			count++
		}
	}
	return found, count == 1
}

// Modules lists the generated modules inside one app.
func (w *Workspace) Modules(app AppTarget) []string {
	entries, err := os.ReadDir(filepath.Join(w.Root, app.SrcPath()))
	if err != nil {
		return nil
	}
	var modules []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		modules = append(modules, entry.Name())
	}
	sort.Strings(modules)
	return modules
}

// AppNames lists every app name.
func (w *Workspace) AppNames() []string {
	names := make([]string, len(w.Apps))
	for i, app := range w.Apps {
		names[i] = app.Name
	}
	return names
}

// Sync writes the detected layout back into .nika.toml so the file matches
// reality and `nika start` gets a per-app run command it can use.
func (w *Workspace) Sync() error {
	config := w.Config
	if w.Microservice {
		config.Workspace.Mode = "microservice"
	} else {
		config.Workspace.Mode = "single"
	}
	config.Workspace.Apps = w.AppNames()
	if config.Workspace.DefaultApp == "" && len(w.Apps) > 0 {
		config.Workspace.DefaultApp = w.preferredDefault()
	}
	if config.Apps == nil {
		config.Apps = map[string]nikaconf.Build{}
	}
	for _, app := range w.Apps {
		if app.Dir == "" {
			continue
		}
		existing := config.Apps[app.Name]
		if strings.TrimSpace(existing.Cmd) == "" {
			existing.Cmd = app.RunCommand()
		}
		config.Apps[app.Name] = existing
	}
	// Drop overrides for apps that no longer exist.
	for name := range config.Apps {
		if w.Find(name) == nil {
			delete(config.Apps, name)
		}
	}
	if w.Microservice {
		if target := w.Find(config.Workspace.DefaultApp); target != nil {
			config.Build.Cmd = target.RunCommand()
		}
	}
	w.Config = config
	return nikaconf.SaveFrom(w.Root, config)
}

// preferredDefault picks the app most likely to be the one the user runs: an
// HTTP gateway named api/gateway/http if present, otherwise the first.
func (w *Workspace) preferredDefault() string {
	for _, preferred := range []string{"api", "gateway", "http", "web", "app"} {
		for _, app := range w.Apps {
			if strings.EqualFold(app.Name, preferred) {
				return app.Name
			}
		}
	}
	if len(w.Apps) > 0 {
		return w.Apps[0].Name
	}
	return ""
}

// SetDefaultApp records the app that `nika start` runs by default.
func (w *Workspace) SetDefaultApp(name string) error {
	target := w.Find(name)
	if target == nil {
		return fmt.Errorf("unknown app %q — available: %s", name, strings.Join(w.AppNames(), ", "))
	}
	w.Config.Workspace.DefaultApp = target.Name
	w.DefaultApp = target.Name
	return w.Sync()
}

// resolveModulePathIn reads the module directive out of dir/go.mod.
func resolveModulePathIn(dir string) (string, error) {
	content, err := common.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("reading go.mod: %w", err)
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}
	return "", fmt.Errorf("no module directive found in go.mod")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// path joins slash-separated path elements, skipping empties.
func path(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, "/")
}
