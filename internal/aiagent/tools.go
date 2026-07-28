package aiagent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nika-framework/nika-cli/internal"
)

// Tool is one capability exposed to the model.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
	// Mutates marks tools that change the project. The UI highlights them and
	// read-only mode refuses them.
	Mutates bool
	Run     func(ctx context.Context, box *Toolbox, args Arguments) (string, error)
}

// Toolbox is the execution environment: a project root every path is confined
// to, plus the policy for running shell commands.
type Toolbox struct {
	// Root is the absolute directory the agent may touch. Every path argument
	// is resolved against it and rejected if it escapes — an agent that can be
	// talked into writing /etc or ~/.ssh is not a code generator.
	Root string
	// AllowCommands extends the default run_command allowlist.
	AllowCommands []string
	// AllowAnyCommand disables the allowlist. Opt-in only.
	AllowAnyCommand bool
	// ReadOnly refuses every mutating tool.
	ReadOnly bool
	// Changed records the files written this session, for the summary.
	Changed map[string]bool
}

// NewToolbox creates a toolbox rooted at dir.
func NewToolbox(dir string) (*Toolbox, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		abs = resolved
	}
	return &Toolbox{Root: abs, Changed: map[string]bool{}}, nil
}

// resolve turns a model-supplied path into an absolute path inside Root.
func (b *Toolbox) resolve(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return b.Root, nil
	}
	if filepath.IsAbs(rel) {
		// Absolute paths are accepted only when they already point inside the
		// project, so a model that echoes back a path we gave it still works.
		cleaned := filepath.Clean(rel)
		if !within(b.Root, cleaned) {
			return "", fmt.Errorf("path %q is outside the project root %s", rel, b.Root)
		}
		return cleaned, nil
	}
	joined := filepath.Clean(filepath.Join(b.Root, rel))
	if !within(b.Root, joined) {
		return "", fmt.Errorf("path %q escapes the project root", rel)
	}
	return joined, nil
}

// within reports whether path is root or lives under it.
func within(root, path string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(os.PathSeparator))
}

// rel renders an absolute path relative to the root, for messages.
func (b *Toolbox) rel(abs string) string {
	if r, err := filepath.Rel(b.Root, abs); err == nil {
		return filepath.ToSlash(r)
	}
	return abs
}

// defaultAllowedCommands is the set of executables run_command accepts without
// extra configuration: build/test/format/inspect, nothing that reaches the
// network or deletes anything.
var defaultAllowedCommands = []string{
	"go", "gofmt", "goimports", "swag", "nika",
	"ls", "cat", "head", "tail", "wc", "grep", "rg", "find", "tree",
	"git status", "git diff", "git log", "git show", "git ls-files", "git branch",
}

// commandAllowed checks a command line against the allowlist.
func (b *Toolbox) commandAllowed(command string) bool {
	if b.AllowAnyCommand {
		return true
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	candidates := append(append([]string{}, defaultAllowedCommands...), b.AllowCommands...)
	for _, allowed := range candidates {
		allowedFields := strings.Fields(allowed)
		if len(allowedFields) == 0 || len(allowedFields) > len(fields) {
			continue
		}
		match := true
		for i, part := range allowedFields {
			if fields[i] != part {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// dangerousShell rejects shell metacharacters. run_command executes without a
// shell, so these would be passed as literal arguments and confuse the model
// far more than an explicit refusal does.
var dangerousShell = regexp.MustCompile("[;&|><`$]")

// Tools returns the full toolset, in the order the model sees them.
func Tools() []Tool {
	return []Tool{
		listDirTool(),
		readFileTool(),
		searchTool(),
		writeFileTool(),
		editFileTool(),
		nikaGenerateTool(),
		projectInfoTool(),
		runCommandTool(),
	}
}

// Schemas renders the toolset for the provider.
func Schemas(tools []Tool) []ToolSchema {
	schemas := make([]ToolSchema, 0, len(tools))
	for _, tool := range tools {
		schemas = append(schemas, ToolSchema{
			Type: "function",
			Function: FunctionSchema{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}
	return schemas
}

// object is a small helper for building JSON Schema fragments.
func object(properties map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

func str(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func boolean(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func integer(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

// ── list_dir ────────────────────────────────────────────────────────

func listDirTool() Tool {
	return Tool{
		Name:        "list_dir",
		Description: "List files and directories at a path inside the project. Use this first to understand the layout before reading or editing files.",
		Parameters: object(map[string]any{
			"path":      str("Directory relative to the project root. Use \".\" for the root."),
			"recursive": boolean("List nested directories too (depth-limited). Default false."),
		}, "path"),
		Run: func(ctx context.Context, box *Toolbox, args Arguments) (string, error) {
			var input struct {
				Path      string `json:"path"`
				Recursive bool   `json:"recursive"`
			}
			if err := args.Decode(&input); err != nil {
				return "", err
			}
			dir, err := box.resolve(input.Path)
			if err != nil {
				return "", err
			}
			if input.Recursive {
				return listRecursive(box, dir)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				return "", err
			}
			var lines []string
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".") && entry.Name() != ".nika.toml" && entry.Name() != ".env" {
					continue
				}
				if entry.IsDir() {
					lines = append(lines, entry.Name()+"/")
					continue
				}
				lines = append(lines, entry.Name())
			}
			if len(lines) == 0 {
				return "(empty directory)", nil
			}
			sort.Strings(lines)
			return strings.Join(lines, "\n"), nil
		},
	}
}

// skipDirs are never walked: they are large, generated, or irrelevant.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "tmp": true,
	".venv": true, "dist": true, "build": true, ".idea": true, ".vscode": true,
}

func listRecursive(box *Toolbox, dir string) (string, error) {
	var lines []string
	const maxEntries = 800
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := entry.Name()
		if entry.IsDir() {
			if skipDirs[name] || (strings.HasPrefix(name, ".") && path != dir) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		lines = append(lines, box.rel(path))
		if len(lines) >= maxEntries {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(lines)
	out := strings.Join(lines, "\n")
	if len(lines) >= maxEntries {
		out += fmt.Sprintf("\n… truncated at %d files", maxEntries)
	}
	return out, nil
}

// ── read_file ───────────────────────────────────────────────────────

func readFileTool() Tool {
	return Tool{
		Name:        "read_file",
		Description: "Read a text file from the project. Always read a file before editing it, so the edit matches the real contents.",
		Parameters: object(map[string]any{
			"path":   str("File path relative to the project root."),
			"offset": integer("1-based first line to return. Optional."),
			"limit":  integer("Maximum number of lines to return. Optional, default 800."),
		}, "path"),
		Run: func(ctx context.Context, box *Toolbox, args Arguments) (string, error) {
			var input struct {
				Path   string `json:"path"`
				Offset int    `json:"offset"`
				Limit  int    `json:"limit"`
			}
			if err := args.Decode(&input); err != nil {
				return "", err
			}
			path, err := box.resolve(input.Path)
			if err != nil {
				return "", err
			}
			file, err := os.Open(path)
			if err != nil {
				return "", err
			}
			defer file.Close()

			limit := input.Limit
			if limit <= 0 {
				limit = 800
			}
			offset := input.Offset
			if offset <= 0 {
				offset = 1
			}

			var out strings.Builder
			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
			line := 0
			printed := 0
			for scanner.Scan() {
				line++
				if line < offset {
					continue
				}
				if printed >= limit {
					fmt.Fprintf(&out, "… file continues past line %d\n", line-1)
					break
				}
				// Line numbers let the model quote an exact region back in an
				// edit_file call.
				fmt.Fprintf(&out, "%d\t%s\n", line, scanner.Text())
				printed++
			}
			if err := scanner.Err(); err != nil {
				return "", err
			}
			if out.Len() == 0 {
				return "(empty file)", nil
			}
			return out.String(), nil
		},
	}
}

// ── search ──────────────────────────────────────────────────────────

func searchTool() Tool {
	return Tool{
		Name:        "search",
		Description: "Search the project for a regular expression and return matching lines with their file and line number. Use it to find a struct, route, or symbol before editing.",
		Parameters: object(map[string]any{
			"pattern": str("Go regular expression to search for."),
			"path":    str("Directory to search under, relative to the project root. Default \".\"."),
			"ext":     str("Only search files with this extension, e.g. \".go\". Optional."),
		}, "pattern"),
		Run: func(ctx context.Context, box *Toolbox, args Arguments) (string, error) {
			var input struct {
				Pattern string `json:"pattern"`
				Path    string `json:"path"`
				Ext     string `json:"ext"`
			}
			if err := args.Decode(&input); err != nil {
				return "", err
			}
			if strings.TrimSpace(input.Pattern) == "" {
				return "", fmt.Errorf("pattern is required")
			}
			re, err := regexp.Compile(input.Pattern)
			if err != nil {
				return "", fmt.Errorf("invalid pattern: %w", err)
			}
			root, err := box.resolve(input.Path)
			if err != nil {
				return "", err
			}

			var matches []string
			const maxMatches = 200
			walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if entry.IsDir() {
					if skipDirs[entry.Name()] || (strings.HasPrefix(entry.Name(), ".") && path != root) {
						return filepath.SkipDir
					}
					return nil
				}
				if input.Ext != "" && filepath.Ext(path) != input.Ext {
					return nil
				}
				info, err := entry.Info()
				if err != nil || info.Size() > 4<<20 {
					return nil
				}
				file, err := os.Open(path)
				if err != nil {
					return nil
				}
				defer file.Close()

				scanner := bufio.NewScanner(file)
				scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
				line := 0
				for scanner.Scan() {
					line++
					text := scanner.Text()
					if re.MatchString(text) {
						matches = append(matches, fmt.Sprintf("%s:%d: %s", box.rel(path), line, strings.TrimSpace(text)))
						if len(matches) >= maxMatches {
							return filepath.SkipAll
						}
					}
				}
				return nil
			})
			if walkErr != nil {
				return "", walkErr
			}
			if len(matches) == 0 {
				return "no matches", nil
			}
			out := strings.Join(matches, "\n")
			if len(matches) >= maxMatches {
				out += fmt.Sprintf("\n… truncated at %d matches", maxMatches)
			}
			return out, nil
		},
	}
}

// ── write_file ──────────────────────────────────────────────────────

func writeFileTool() Tool {
	return Tool{
		Name:        "write_file",
		Description: "Create a file or replace its entire contents. For a change to an existing file prefer edit_file, which cannot accidentally drop the rest of the file.",
		Mutates:     true,
		Parameters: object(map[string]any{
			"path":    str("File path relative to the project root."),
			"content": str("The complete new contents of the file."),
		}, "path", "content"),
		Run: func(ctx context.Context, box *Toolbox, args Arguments) (string, error) {
			var input struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := args.Decode(&input); err != nil {
				return "", err
			}
			path, err := box.resolve(input.Path)
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return "", err
			}
			existed := fileExists(path)
			if err := os.WriteFile(path, []byte(input.Content), 0o644); err != nil {
				return "", err
			}
			box.Changed[box.rel(path)] = true
			if existed {
				return fmt.Sprintf("overwrote %s (%d bytes)", box.rel(path), len(input.Content)), nil
			}
			return fmt.Sprintf("created %s (%d bytes)", box.rel(path), len(input.Content)), nil
		},
	}
}

// ── edit_file ───────────────────────────────────────────────────────

func editFileTool() Tool {
	return Tool{
		Name:        "edit_file",
		Description: "Replace an exact string in a file. old_string must appear verbatim, including indentation, and must be unique unless replace_all is true. This is the right tool for adding a field to a struct or changing one line.",
		Mutates:     true,
		Parameters: object(map[string]any{
			"path":        str("File path relative to the project root."),
			"old_string":  str("Exact text to replace. Include surrounding lines to make it unique. Do not include the line-number prefixes shown by read_file."),
			"new_string":  str("Replacement text."),
			"replace_all": boolean("Replace every occurrence instead of requiring a unique match."),
		}, "path", "old_string", "new_string"),
		Run: func(ctx context.Context, box *Toolbox, args Arguments) (string, error) {
			var input struct {
				Path      string `json:"path"`
				OldString string `json:"old_string"`
				// A pointer so an omitted new_string is distinguishable from
				// an explicit empty one. Treating "absent" as "" turns a
				// half-formed call into a silent deletion — which is exactly
				// what a small model produces when it runs out of output
				// budget mid-object.
				NewString  *string `json:"new_string"`
				ReplaceAll bool    `json:"replace_all"`
			}
			if err := args.Decode(&input); err != nil {
				return "", err
			}
			if input.NewString == nil {
				return "", fmt.Errorf("new_string is required — send old_string and new_string together in one call. " +
					"To delete the text, pass new_string as an empty string explicitly")
			}
			newString := *input.NewString

			path, err := box.resolve(input.Path)
			if err != nil {
				return "", err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			source := string(raw)
			if input.OldString == "" {
				return "", fmt.Errorf("old_string is required; use write_file to create a file")
			}

			count := strings.Count(source, input.OldString)
			if count > 1 && !input.ReplaceAll {
				return "", fmt.Errorf("old_string appears %d times in %s — add surrounding context to make it unique, or set replace_all", count, box.rel(path))
			}

			var updated, note string
			switch {
			case count == 1:
				updated = strings.Replace(source, input.OldString, newString, 1)
			case count > 1:
				updated = strings.ReplaceAll(source, input.OldString, newString)
			default:
				// No exact match. Retry ignoring whitespace differences before
				// giving up: smaller models reproduce a line's words reliably
				// but its exact spacing and full tag text far less so, and
				// failing here strands an otherwise correct edit.
				fuzzy, matches, err := replaceIgnoringWhitespace(source, input.OldString, newString, input.ReplaceAll)
				if err != nil {
					return "", editNotFoundError(box.rel(path), source, input.OldString, err)
				}
				updated, count, note = fuzzy, matches, " (matched ignoring whitespace)"
			}

			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				return "", err
			}
			box.Changed[box.rel(path)] = true
			return fmt.Sprintf("edited %s (%d replacement(s))%s", box.rel(path), count, note), nil
		},
	}
}

// ── project_info ────────────────────────────────────────────────────

func projectInfoTool() Tool {
	return Tool{
		Name:        "project_info",
		Description: "Describe the Nika project: module path, whether it is a microservice workspace, the apps it contains, and the modules inside each app. Call this before generating or editing modules.",
		Parameters:  object(map[string]any{}),
		Run: func(ctx context.Context, box *Toolbox, args Arguments) (string, error) {
			workspace, err := internal.LoadWorkspaceAt(box.Root)
			if err != nil {
				return "", err
			}
			var out strings.Builder
			fmt.Fprintf(&out, "module path: %s\n", workspace.ModulePath)
			fmt.Fprintf(&out, "layout: %s\n", map[bool]string{true: "microservice workspace", false: "single app"}[workspace.Microservice])
			if workspace.DefaultApp != "" {
				fmt.Fprintf(&out, "default app: %s\n", workspace.DefaultApp)
			}
			fmt.Fprintln(&out, "apps:")
			for _, app := range workspace.Apps {
				fmt.Fprintf(&out, "  - name: %s\n    src: %s\n    import prefix: %s/%s\n",
					app.Name, app.SrcDir, workspace.ModulePath, app.SrcImport())
				if app.MainGo != "" {
					fmt.Fprintf(&out, "    main: %s\n", app.MainGo)
				}
				modules := workspace.Modules(app)
				if len(modules) > 0 {
					fmt.Fprintf(&out, "    modules: %s\n", strings.Join(modules, ", "))
				}
			}
			return out.String(), nil
		},
	}
}

// ── nika_generate ───────────────────────────────────────────────────

func nikaGenerateTool() Tool {
	return Tool{
		Name: "nika_generate",
		Description: "Scaffold a complete Nika resource (model, repository, DTOs, service, controller, response mapper, module registration) using the CLI's own templates. " +
			"Prefer this over writing those files by hand: it produces code that matches the framework exactly. " +
			"For changes to an existing module use read_file and edit_file instead.",
		Mutates: true,
		Parameters: object(map[string]any{
			"module":   str("Lowercase English module name, e.g. \"product\"."),
			"database": str("One of: mongodb, postgres, mysql, sqlite."),
			"app":      str("Which app/microservice to generate into. Required in a workspace; get the names from project_info."),
			"fields": map[string]any{
				"type":        "array",
				"description": "The model's fields, excluding ID/CreatedAt/UpdatedAt which are always added.",
				"items": object(map[string]any{
					"name":     str("snake_case field name."),
					"type":     str("Go type. MongoDB: string, int, int64, float64, bool, time.Time, primitive.ObjectID, []string, map[string]any. SQL: string, int, int64, float64, bool, time.Time."),
					"required": boolean("Whether the field is required."),
				}, "name", "type"),
			},
		}, "module", "database", "fields"),
		Run: func(ctx context.Context, box *Toolbox, args Arguments) (string, error) {
			var input struct {
				Module   string `json:"module"`
				Database string `json:"database"`
				App      string `json:"app"`
				Fields   []struct {
					Name     string `json:"name"`
					Type     string `json:"type"`
					Required bool   `json:"required"`
				} `json:"fields"`
			}
			if err := args.Decode(&input); err != nil {
				return "", err
			}
			if len(input.Fields) == 0 {
				return "", fmt.Errorf("fields is required and must not be empty")
			}

			workspace, err := internal.LoadWorkspaceAt(box.Root)
			if err != nil {
				return "", err
			}
			// The generator prompts on stdin when the app is ambiguous, and
			// there is no stdin behind a browser chat — so resolve it here and
			// return a question the model can relay instead.
			if input.App == "" {
				if len(workspace.Apps) != 1 {
					return "", fmt.Errorf("this project has several apps (%s) — pass the app argument to say which one",
						strings.Join(workspace.AppNames(), ", "))
				}
				input.App = workspace.Apps[0].Name
			}

			database := internal.ParseDatabaseType(input.Database)
			if database == "" {
				return "", fmt.Errorf("unsupported database %q (use mongodb, postgres, mysql, or sqlite)", input.Database)
			}
			fields := make([]internal.Field, 0, len(input.Fields))
			for _, field := range input.Fields {
				built, err := internal.NewField(field.Name, field.Type, field.Required, database)
				if err != nil {
					return "", err
				}
				fields = append(fields, built)
			}

			restore, err := chdir(box.Root)
			if err != nil {
				return "", err
			}
			defer restore()

			if err := internal.RunGenerate(&internal.GenerateConfig{
				Type:     internal.GenResource,
				Module:   input.Module,
				Database: string(database),
				App:      input.App,
				Fields:   fields,
			}); err != nil {
				return "", err
			}
			target := workspace.Find(input.App)
			srcDir := "src"
			if target != nil {
				srcDir = target.SrcDir
			}
			box.Changed[srcDir+"/"+input.Module] = true
			return fmt.Sprintf("generated module %q in %s/%s and registered it in the app module", input.Module, srcDir, input.Module), nil
		},
	}
}

// chdir switches the process working directory and returns a restore func.
// RunGenerate resolves paths relative to the process CWD, and the browser
// agent may be serving a different directory than the one it was started in.
func chdir(dir string) (func(), error) {
	previous, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if err := os.Chdir(dir); err != nil {
		return nil, err
	}
	return func() { _ = os.Chdir(previous) }, nil
}

// ── run_command ─────────────────────────────────────────────────────

func runCommandTool() Tool {
	return Tool{
		Name: "run_command",
		Description: "Run a build or inspection command in the project root and return its combined output. " +
			"Use `go build ./...` after editing Go files to verify the change compiles. " +
			"Only a safe allowlist is permitted (go, gofmt, git status/diff, nika, ls, grep …); shell operators are not supported.",
		Mutates: true,
		Parameters: object(map[string]any{
			"command": str("The command line to run, e.g. \"go build ./...\"."),
		}, "command"),
		Run: func(ctx context.Context, box *Toolbox, args Arguments) (string, error) {
			var input struct {
				Command string `json:"command"`
			}
			if err := args.Decode(&input); err != nil {
				return "", err
			}
			command := strings.TrimSpace(input.Command)
			if command == "" {
				return "", fmt.Errorf("command is required")
			}
			if dangerousShell.MatchString(command) {
				return "", fmt.Errorf("shell operators (; & | > < ` $) are not supported — run one plain command at a time")
			}
			if !box.commandAllowed(command) {
				return "", fmt.Errorf("command %q is not allowed; permitted prefixes: %s (extend with allow_commands in %s)",
					command, strings.Join(defaultAllowedCommands, ", "), ".nika.toml")
			}

			timeout, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			fields := strings.Fields(command)
			cmd := exec.CommandContext(timeout, fields[0], fields[1:]...)
			cmd.Dir = box.Root
			output, err := cmd.CombinedOutput()
			text := strings.TrimSpace(string(output))
			if len(text) > 20000 {
				text = text[:20000] + "\n… output truncated"
			}
			if err != nil {
				// A failing build is information the model needs, not a tool
				// failure — return it as content so it can fix the code.
				if text == "" {
					return "", fmt.Errorf("%s: %w", command, err)
				}
				return fmt.Sprintf("command failed (%v):\n%s", err, text), nil
			}
			if text == "" {
				return "command succeeded with no output", nil
			}
			return text, nil
		},
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// jsonError renders a tool error as content the model can act on.
func jsonError(err error) string {
	payload, _ := json.Marshal(map[string]string{"error": err.Error()})
	return string(payload)
}
