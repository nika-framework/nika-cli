package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// scaffold writes a project tree from a path→content map and returns its root.
func scaffold(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLoadWorkspaceDetectsSingleApp(t *testing.T) {
	root := scaffold(t, map[string]string{
		"go.mod":            "module example.com/app\n\ngo 1.23\n",
		"main.go":           "package main\n",
		"src/user/dummy.go": "package user\n",
	})

	workspace, err := LoadWorkspaceAt(root)
	if err != nil {
		t.Fatalf("LoadWorkspaceAt() error = %v", err)
	}
	if workspace.Microservice {
		t.Error("single-app project detected as a workspace")
	}
	if len(workspace.Apps) != 1 {
		t.Fatalf("apps = %d, want 1", len(workspace.Apps))
	}
	app := workspace.Apps[0]
	if app.SrcDir != "src" || app.SrcImport() != "src" {
		t.Errorf("src = %q", app.SrcDir)
	}
	if app.RunCommand() != "go run ." {
		t.Errorf("run command = %q", app.RunCommand())
	}
}

// TestLoadWorkspaceDetectsMicroservices is the layout that used to break
// generation entirely: modules belong under apps/<service>/src, not src/.
func TestLoadWorkspaceDetectsMicroservices(t *testing.T) {
	root := scaffold(t, map[string]string{
		"go.mod":                          "module nikaapp\n\ngo 1.23\n",
		"apps/api/main.go":                "package main\n",
		"apps/api/src/app.module.go":      "package src\n",
		"apps/micro-grpc/main.go":         "package main\n",
		"apps/micro-grpc/src/user/x.go":   "package user\n",
		"apps/api/src/user-grpc/y.go":     "package usergrpc\n",
		"apps/notes/README.md":            "not an app\n",
		"apps/micro-grpc/src/order/z.go":  "package order\n",
		"internal/common/guards/guard.go": "package guards\n",
	})

	workspace, err := LoadWorkspaceAt(root)
	if err != nil {
		t.Fatalf("LoadWorkspaceAt() error = %v", err)
	}
	if !workspace.Microservice {
		t.Fatal("workspace not detected")
	}
	if got := workspace.AppNames(); len(got) != 2 || got[0] != "api" || got[1] != "micro-grpc" {
		// apps/notes has neither main.go nor src/, so it is not an app.
		t.Fatalf("apps = %v, want [api micro-grpc]", got)
	}

	grpc := workspace.Find("micro-grpc")
	if grpc == nil {
		t.Fatal("micro-grpc not found")
	}
	if grpc.SrcImport() != "apps/micro-grpc/src" {
		t.Errorf("src import = %q", grpc.SrcImport())
	}
	if grpc.RunCommand() != "go run ./apps/micro-grpc/main.go" {
		t.Errorf("run command = %q", grpc.RunCommand())
	}
	if got := grpc.ModuleDir("order"); got != filepath.Join("apps", "micro-grpc", "src", "order") {
		t.Errorf("module dir = %q", got)
	}
	if modules := workspace.Modules(*grpc); len(modules) != 2 || modules[0] != "order" || modules[1] != "user" {
		t.Errorf("modules = %v", modules)
	}
}

// TestFindMatchesPartialNames: users type "grpc", not "micro-grpc".
func TestFindMatchesPartialNames(t *testing.T) {
	root := scaffold(t, map[string]string{
		"go.mod":                  "module nikaapp\n",
		"apps/api/main.go":        "package main\n",
		"apps/micro-grpc/main.go": "package main\n",
		"apps/micro-tcp/main.go":  "package main\n",
	})
	workspace, err := LoadWorkspaceAt(root)
	if err != nil {
		t.Fatal(err)
	}

	if app := workspace.Find("grpc"); app == nil || app.Name != "micro-grpc" {
		t.Errorf("Find(\"grpc\") = %v", app)
	}
	if app := workspace.Find("api"); app == nil || app.Name != "api" {
		t.Errorf("Find(\"api\") = %v", app)
	}
	// "micro" matches two apps: ambiguous must not silently pick one.
	if app := workspace.Find("micro"); app != nil {
		t.Errorf("Find(\"micro\") = %v, want nil for an ambiguous prefix", app)
	}
}

// TestSelectAppUsesFlagAndEnv covers the non-interactive paths.
func TestSelectAppUsesFlagAndEnv(t *testing.T) {
	root := scaffold(t, map[string]string{
		"go.mod":                  "module nikaapp\n",
		"apps/api/main.go":        "package main\n",
		"apps/micro-grpc/main.go": "package main\n",
	})
	workspace, err := LoadWorkspaceAt(root)
	if err != nil {
		t.Fatal(err)
	}

	app, err := workspace.SelectApp("micro-grpc")
	if err != nil || app.Name != "micro-grpc" {
		t.Fatalf("SelectApp(flag) = %v, %v", app.Name, err)
	}

	t.Setenv("NIKA_APP", "api")
	app, err = workspace.SelectApp("")
	if err != nil || app.Name != "api" {
		t.Fatalf("SelectApp(env) = %v, %v", app.Name, err)
	}

	if _, err := workspace.SelectApp("nope"); err == nil {
		t.Error("unknown app was accepted")
	}
}

// TestFindModuleLocatesTheOwningApp, and refuses when the name is shared.
func TestFindModule(t *testing.T) {
	root := scaffold(t, map[string]string{
		"go.mod":                         "module nikaapp\n",
		"apps/api/src/user/x.go":         "package user\n",
		"apps/micro-grpc/src/user/x.go":  "package user\n",
		"apps/micro-grpc/src/order/x.go": "package order\n",
	})
	workspace, err := LoadWorkspaceAt(root)
	if err != nil {
		t.Fatal(err)
	}

	app, ok := workspace.FindModule("order")
	if !ok || app.Name != "micro-grpc" {
		t.Errorf("FindModule(order) = %v, %v", app.Name, ok)
	}
	if _, ok := workspace.FindModule("user"); ok {
		t.Error("FindModule(user) resolved a name that exists in two apps")
	}
	if _, ok := workspace.FindModule("missing"); ok {
		t.Error("FindModule(missing) resolved")
	}
}

// TestSyncWritesRunCommands: the point of syncing is that `nika start` stops
// running "go run ." at a root with no main package.
func TestSyncWritesRunCommands(t *testing.T) {
	root := scaffold(t, map[string]string{
		"go.mod":                  "module nikaapp\n",
		"apps/api/main.go":        "package main\n",
		"apps/micro-grpc/main.go": "package main\n",
	})
	workspace, err := LoadWorkspaceAt(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	reloaded, err := LoadWorkspaceAt(root)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Config.Workspace.Mode != "microservice" {
		t.Errorf("mode = %q", reloaded.Config.Workspace.Mode)
	}
	// "api" is preferred as the default gateway.
	if reloaded.Config.Workspace.DefaultApp != "api" {
		t.Errorf("default app = %q", reloaded.Config.Workspace.DefaultApp)
	}
	if reloaded.Config.Build.Cmd != "go run ./apps/api/main.go" {
		t.Errorf("build cmd = %q", reloaded.Config.Build.Cmd)
	}
	if got := reloaded.Config.BuildFor("micro-grpc").Cmd; got != "go run ./apps/micro-grpc/main.go" {
		t.Errorf("micro-grpc cmd = %q", got)
	}
}
