package start

import (
	"fmt"
	"strings"

	"github.com/nika-framework/nika-cli/internal"
	"github.com/nika-framework/nika-cli/internal/nikaconf"
)

// StartApp is one `nika start` invocation.
type StartApp struct {
	// Target is an explicit file or directory to run. When set it wins over
	// everything in .nika.toml.
	Target string
	// App names the microservice to start. Empty means the configured default,
	// or an interactive pick when the workspace has several and none is set.
	App string
	// WatchMode restarts the process on file changes.
	WatchMode bool
	// PathTom is the config file path.
	PathTom string
}

func NewStartApp(target string, watchMode bool) *StartApp {
	return &StartApp{
		Target:    target,
		WatchMode: watchMode,
		PathTom:   nikaconf.FileName,
	}
}

// NewStartAppFor builds a runner bound to a specific app.
func NewStartAppFor(target, app string, watchMode bool) *StartApp {
	start := NewStartApp(target, watchMode)
	start.App = app
	return start
}

// plan is the resolved answer to "what command runs, and from where".
type plan struct {
	Config Config
	Build  BuildConfig
	App    string
}

// resolve merges the config, the workspace layout and the CLI flags into one
// command.
//
// The precedence — explicit target, then --app, then default_app, then the
// root [build] cmd — exists because a microservice workspace has no single
// correct answer. Running `go run .` at the root of a project whose main
// packages all live under apps/ is the failure this replaces.
func (a StartApp) resolve() (plan, error) {
	config, err := a.LoadConfig()
	if err != nil {
		return plan{}, fmt.Errorf("failed to load config: %w", err)
	}

	// An explicit path argument bypasses the workspace entirely.
	if target := strings.TrimSpace(a.Target); target != "" && target != "." && target != "./main.go" {
		build := config.Build
		build.Cmd = "go run ./" + strings.TrimPrefix(strings.TrimPrefix(target, "./"), "/")
		build.Args = nil
		return plan{Config: config, Build: build}, nil
	}

	workspace, err := internal.LoadWorkspace()
	if err != nil {
		// Not a Go project, or no go.mod: fall back to the plain config so
		// `nika start` still works for anything the user wired up by hand.
		return plan{Config: config, Build: config.Build}, nil
	}

	requested := a.App
	if requested == "" {
		requested = config.Workspace.DefaultApp
	}
	app, err := workspace.SelectApp(requested)
	if err != nil {
		return plan{}, err
	}

	build := config.BuildFor(app.Name)
	// A workspace app whose command still says "go run ." would start the
	// wrong package, so derive it from the app's main.go instead.
	if strings.TrimSpace(build.Cmd) == "" || (app.Dir != "" && build.Cmd == "go run .") {
		build.Cmd = app.RunCommand()
	}
	return plan{Config: config, Build: build, App: app.Name}, nil
}
