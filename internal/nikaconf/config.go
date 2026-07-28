// Package nikaconf owns the on-disk .nika.toml format.
//
// It lives in its own package because three consumers need the same struct:
// the generator (which asks it which app a module belongs to), the watcher in
// internal/start, and the AI agent. Keeping one decoder means a `nika g` run
// can no longer round-trip the file and silently drop the [build] section that
// only `nika start` cares about.
package nikaconf

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// FileName is the config file every Nika project carries at its root.
const FileName = ".nika.toml"

// Config is the whole .nika.toml document.
type Config struct {
	Root        string           `toml:"root"`
	TestdataDir string           `toml:"testdata_dir"`
	TmpDir      string           `toml:"tmp_dir"`
	Workspace   WorkspaceConfig  `toml:"workspace"`
	Agent       AgentConfig      `toml:"agent"`
	Build       BuildConfig      `toml:"build"`
	Apps        map[string]Build `toml:"apps"`
}

// WorkspaceConfig describes the project layout: one app at the root, or several
// under an apps/ directory.
type WorkspaceConfig struct {
	// Mode is "single" or "microservice". Empty means "detect from disk".
	Mode string `toml:"mode"`
	// AppsDir is the parent of the per-service directories (default "apps").
	AppsDir string `toml:"apps_dir"`
	// Apps lists the service names in apps_dir. Empty means "detect from disk".
	Apps []string `toml:"apps"`
	// DefaultApp is used by `nika start` and pre-selected in prompts.
	DefaultApp string `toml:"default_app"`
	// SrcDir is the module folder name inside an app (default "src").
	SrcDir string `toml:"src_dir"`
}

// Build is the per-app override of the root [build] section. Only Cmd and Env
// are usually set; anything empty falls back to [build].
type Build struct {
	Cmd     string            `toml:"cmd"`
	Args    []string          `toml:"args"`
	Bin     string            `toml:"bin"`
	PreCmd  []string          `toml:"pre_cmd"`
	PostCmd []string          `toml:"post_cmd"`
	Env     map[string]string `toml:"env"`
}

// BuildConfig holds build and watch related settings.
type BuildConfig struct {
	Cmd          string            `toml:"cmd"`
	Args         []string          `toml:"args"`
	Bin          string            `toml:"bin"`
	Delay        int               `toml:"delay"`
	ExcludeDir   []string          `toml:"exclude_dir"`
	ExcludeFile  []string          `toml:"exclude_file"`
	ExcludeRegex []string          `toml:"exclude_regex"`
	IncludeExt   []string          `toml:"include_ext"`
	PreCmd       []string          `toml:"pre_cmd"`
	PostCmd      []string          `toml:"post_cmd"`
	Env          map[string]string `toml:"env"`
	EnvFiles     []string          `toml:"env_files"`
}

// AgentConfig is the AI provider configuration. API keys are referenced by
// environment variable name and never written to disk.
type AgentConfig struct {
	Provider  string `toml:"provider"`
	Model     string `toml:"model"`
	BaseURL   string `toml:"base_url"`
	APIKeyEnv string `toml:"api_key_env"`
	// MaxSteps caps the agent loop's tool-call rounds (default 25).
	MaxSteps int `toml:"max_steps"`
	// AllowCommands extends the run_command allowlist with extra prefixes.
	AllowCommands []string `toml:"allow_commands"`
}

// Default returns the config written for a project that has none.
func Default() Config {
	return Config{
		Root:        ".",
		TestdataDir: "testdata",
		TmpDir:      "tmp",
		// Mode is left empty on purpose: empty means "detect from disk", so a
		// project that later grows an apps/ directory is picked up without
		// anyone having to remember to edit this file.
		Workspace: WorkspaceConfig{AppsDir: "apps", SrcDir: "src"},
		Build: BuildConfig{
			Cmd:          "go run .",
			Args:         []string{},
			Bin:          "",
			Delay:        1000,
			ExcludeDir:   []string{"docs", "tmp", "vendor", "testdata", ".git", "cache"},
			ExcludeFile:  []string{},
			ExcludeRegex: []string{`^\.`},
			IncludeExt:   []string{".go"},
			PreCmd:       []string{},
			PostCmd:      []string{},
			Env:          map[string]string{},
			EnvFiles:     []string{},
		},
	}
}

// Warnings holds the corrections Normalize applied to the last load. Commands
// print them once so a malformed setting is visible rather than silent.
var lastWarnings []string

// TakeWarnings returns and clears the warnings from the most recent load.
func TakeWarnings() []string {
	warnings := lastWarnings
	lastWarnings = nil
	return warnings
}

// Load reads path. A missing file is not an error: the defaults come back with
// exists=false so callers can decide whether to write one.
func Load(path string) (Config, bool, error) {
	if path == "" {
		path = FileName
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Default(), false, nil
	} else if err != nil {
		return Default(), false, err
	}
	// Decode into a zero value, not into Default(): a default that the file
	// does not mention must stay indistinguishable from "unset", otherwise
	// Normalize cannot tell an explicit choice from a leftover default.
	var config Config
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return Default(), true, fmt.Errorf("read %s: %w", path, err)
	}
	lastWarnings = config.Normalize()
	return config, true, nil
}

// LoadFrom reads the config for a project rooted at dir.
func LoadFrom(dir string) (Config, bool, error) {
	return Load(filepath.Join(dir, FileName))
}

// Normalize fills in the values that older config files predate, so code
// reading the struct never has to repeat the same empty-string fallbacks.
//
// It returns a warning for every value it had to correct. Silently accepting a
// malformed one is worse than it sounds: `src_dir = "apps/api/"` — a natural
// guess, since it does describe where the source lives — used to produce paths
// like apps/micro-grpc/apps/api//product with no error at all.
func (c *Config) Normalize() []string {
	var warnings []string

	if strings.TrimSpace(c.Root) == "" {
		c.Root = "."
	}
	if strings.TrimSpace(c.TmpDir) == "" {
		c.TmpDir = "tmp"
	}
	if strings.TrimSpace(c.TestdataDir) == "" {
		c.TestdataDir = "testdata"
	}

	c.Workspace.AppsDir, warnings = normalizeFolderName(
		c.Workspace.AppsDir, "apps", "workspace.apps_dir", warnings)
	c.Workspace.SrcDir, warnings = normalizeFolderName(
		c.Workspace.SrcDir, "src", "workspace.src_dir", warnings)
	c.Workspace.Mode = strings.ToLower(strings.TrimSpace(c.Workspace.Mode))
	c.Agent.Provider = strings.ToLower(strings.TrimSpace(c.Agent.Provider))
	if strings.TrimSpace(c.Build.Cmd) == "" {
		c.Build.Cmd = "go run ."
	}
	if c.Build.Delay <= 0 {
		c.Build.Delay = 1000
	}
	if len(c.Build.IncludeExt) == 0 {
		c.Build.IncludeExt = []string{".go"}
	}
	if len(c.Build.ExcludeDir) == 0 {
		c.Build.ExcludeDir = []string{"docs", "tmp", "vendor", "testdata", ".git", "cache"}
	}
	if c.Agent.MaxSteps <= 0 {
		c.Agent.MaxSteps = 25
	}
	return warnings
}

// normalizeFolderName enforces that a setting is a single folder name rather
// than a path, falling back to the default and explaining why.
func normalizeFolderName(value, fallback, key string, warnings []string) (string, []string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, warnings
	}
	cleaned := strings.Trim(strings.ReplaceAll(trimmed, "\\", "/"), "/")
	if cleaned == "" || strings.Contains(cleaned, "/") || cleaned == "." || cleaned == ".." {
		return fallback, append(warnings, fmt.Sprintf(
			"%s in %s must be a single folder name, not a path — got %q. Using %q instead.",
			key, FileName, trimmed, fallback))
	}
	return cleaned, warnings
}

// Save writes the config back, preserving the schema hint at the top.
func Save(path string, config Config) error {
	if path == "" {
		path = FileName
	}
	config.Normalize()
	// Encode into a temp file first so a failed write cannot truncate a
	// working config.
	tmp := path + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}
	if _, err := file.WriteString("#:schema https://json.schemastore.org/any.json\n\n"); err != nil {
		file.Close()
		os.Remove(tmp)
		return err
	}
	if err := toml.NewEncoder(file).Encode(config); err != nil {
		file.Close()
		os.Remove(tmp)
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// SaveFrom writes the config for a project rooted at dir.
func SaveFrom(dir string, config Config) error {
	return Save(filepath.Join(dir, FileName), config)
}

// BuildFor merges the root [build] section with the [apps.<name>] override.
func (c Config) BuildFor(app string) BuildConfig {
	build := c.Build
	override, ok := c.Apps[app]
	if !ok {
		return build
	}
	if strings.TrimSpace(override.Cmd) != "" {
		build.Cmd = override.Cmd
	}
	if len(override.Args) > 0 {
		build.Args = override.Args
	}
	if strings.TrimSpace(override.Bin) != "" {
		build.Bin = override.Bin
	}
	if len(override.PreCmd) > 0 {
		build.PreCmd = override.PreCmd
	}
	if len(override.PostCmd) > 0 {
		build.PostCmd = override.PostCmd
	}
	if len(override.Env) > 0 {
		merged := make(map[string]string, len(build.Env)+len(override.Env))
		for k, v := range build.Env {
			merged[k] = v
		}
		for k, v := range override.Env {
			merged[k] = v
		}
		build.Env = merged
	}
	return build
}

// SetAppBuild records a per-app build command.
func (c *Config) SetAppBuild(app string, build Build) {
	if c.Apps == nil {
		c.Apps = map[string]Build{}
	}
	c.Apps[app] = build
}

// AppNames returns the configured app names in a stable order.
func (c Config) AppNames() []string {
	names := append([]string(nil), c.Workspace.Apps...)
	sort.Strings(names)
	return names
}
