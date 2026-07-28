package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nika-framework/nika-cli/internal/nikaconf"
	"github.com/nika-framework/nika-cli/templates"
)

func TestMessageSubject(t *testing.T) {
	cases := map[string]string{
		"api":           "api",
		"grpc-micro":    "grpc_micro",
		"orders.worker": "orders_worker",
		"Orders-Worker": "orders_worker",
		"a--b":          "a_b",
		"-edge-":        "edge",
	}
	for input, want := range cases {
		if got := messageSubject(input); got != want {
			t.Errorf("messageSubject(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFindMicroTransport(t *testing.T) {
	cases := map[string]string{
		"grpc":     "grpc",
		"GRPC":     "grpc",
		"rabbitmq": "rabbit",
		"amqp":     "rabbit",
		"redismq":  "redis",
	}
	for input, want := range cases {
		transport, ok := FindMicroTransport(input)
		if !ok {
			t.Errorf("FindMicroTransport(%q) not found", input)
			continue
		}
		if transport.Name != want {
			t.Errorf("FindMicroTransport(%q) = %q, want %q", input, transport.Name, want)
		}
	}
	if _, ok := FindMicroTransport("mqtt"); ok {
		t.Error("FindMicroTransport(mqtt) should not resolve")
	}
}

// TestMicroTransportsHaveTemplates guards the one failure that only shows up at
// the moment a user runs the command: a catalogue entry whose main.go template
// was never added to templates/micro.
func TestMicroTransportsHaveTemplates(t *testing.T) {
	for _, transport := range MicroTransports() {
		name := "micro/" + transport.Pkg + "/main.go.tpl"
		if _, err := templates.Read(name); err != nil {
			t.Errorf("transport %q: %v", transport.Name, err)
		}
		if transport.DefaultAppName() != transport.Name+"-micro" {
			t.Errorf("transport %q: unexpected default app name %q", transport.Name, transport.DefaultAppName())
		}
		if len(transport.Env(transport.DefaultAppName())) == 0 {
			t.Errorf("transport %q declares no environment variables", transport.Name)
		}
	}
}

func TestMicroLayoutIgnoresAppsDirEqualToSrcDir(t *testing.T) {
	// Older versions of `nika app sync` wrote apps_dir = "src" for single-app
	// projects. Trusting it would make init try to move src/ into src/api/src.
	workspace := &Workspace{
		Config: nikaconf.Config{
			Workspace: nikaconf.WorkspaceConfig{AppsDir: "src", SrcDir: "src"},
		},
	}
	appsDir, srcName := microLayout(workspace)
	if appsDir != "apps" || srcName != "src" {
		t.Errorf("microLayout() = (%q, %q), want (apps, src)", appsDir, srcName)
	}

	// An existing workspace is described by disk, not by the file.
	existing := &Workspace{
		Microservice: true,
		Apps:         []AppTarget{{Name: "api", Dir: "services/api", SrcDir: "services/api/src"}},
		Config: nikaconf.Config{
			Workspace: nikaconf.WorkspaceConfig{AppsDir: "apps", SrcDir: "src"},
		},
	}
	if appsDir, _ := microLayout(existing); appsDir != "services" {
		t.Errorf("microLayout() apps dir = %q, want services", appsDir)
	}
}

func TestRewriteModuleImports(t *testing.T) {
	root := t.TempDir()

	write := func(name, content string) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(name string) string {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}

	write("apps/api/main.go", `package main

import (
	_ "nikaapp/docs"
	"nikaapp/src"
	"nikaapp/src/common/guards"
	"nikaapp/srcutil"
	"nikaapp/internal/database"
	"other/src"
)
`)
	write("cmd/migrate/main.go", "package main\n\nimport _ \"nikaapp/internal/database/migrations\"\n")
	write("README.md", "nikaapp/src should not be touched\n")

	changed, err := rewriteModuleImports(root, "nikaapp", map[string]string{
		"src":  "apps/api/src",
		"docs": "apps/api/docs",
	})
	if err != nil {
		t.Fatalf("rewriteModuleImports() error = %v", err)
	}
	if len(changed) != 1 || changed[0] != "apps/api/main.go" {
		t.Fatalf("changed = %v, want [apps/api/main.go]", changed)
	}

	main := read("apps/api/main.go")
	for _, want := range []string{
		`_ "nikaapp/apps/api/docs"`,
		`"nikaapp/apps/api/src"`,
		`"nikaapp/apps/api/src/common/guards"`,
	} {
		if !strings.Contains(main, want) {
			t.Errorf("main.go missing %s:\n%s", want, main)
		}
	}
	// A package whose name merely starts with "src" is not the src folder, and
	// another module's src is not ours.
	for _, keep := range []string{`"nikaapp/srcutil"`, `"other/src"`, `"nikaapp/internal/database"`} {
		if !strings.Contains(main, keep) {
			t.Errorf("main.go should still contain %s:\n%s", keep, main)
		}
	}
	if read("README.md") != "nikaapp/src should not be touched\n" {
		t.Error("non-Go files must not be rewritten")
	}
}

func TestAppendEnvFileSkipsDefinedKeys(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".env")
	if err := os.WriteFile(path, []byte("PORT=3007\n# GRPC_ADDR=:1\nGRPC_TARGET=already-set\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	transport, _ := FindMicroTransport("grpc")
	added, err := appendEnvFile(path, "grpc-micro", transport, transport.Env("grpc-micro"))
	if err != nil {
		t.Fatalf("appendEnvFile() error = %v", err)
	}
	if contains(added, "GRPC_TARGET") {
		t.Errorf("GRPC_TARGET was already set and must not be re-added: %v", added)
	}
	// A commented-out assignment is not a definition.
	if !contains(added, "GRPC_ADDR") {
		t.Errorf("GRPC_ADDR is only present as a comment and should have been added: %v", added)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.HasPrefix(text, "PORT=3007\n") {
		t.Errorf("existing content was not preserved:\n%s", text)
	}
	if strings.Count(text, "\nGRPC_TARGET=") != 1 {
		t.Errorf("GRPC_TARGET appears more than once:\n%s", text)
	}

	// Re-running must be a no-op now that every key is defined.
	again, err := appendEnvFile(path, "grpc-micro", transport, transport.Env("grpc-micro"))
	if err != nil {
		t.Fatalf("appendEnvFile() second call error = %v", err)
	}
	if len(again) != 0 {
		t.Errorf("second run added %v, want nothing", again)
	}
}

func TestValidateAppName(t *testing.T) {
	for _, name := range []string{"api", "grpc-micro", "orders_worker", "a1"} {
		if err := validateAppName(name); err != nil {
			t.Errorf("validateAppName(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range []string{"", "Api", "1api", "api/x", "apps", "internal", "cmd"} {
		if err := validateAppName(name); err == nil {
			t.Errorf("validateAppName(%q) = nil, want an error", name)
		}
	}
}
