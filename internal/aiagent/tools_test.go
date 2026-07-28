package aiagent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runTool(t *testing.T, box *Toolbox, name, args string) (string, error) {
	t.Helper()
	for _, tool := range Tools() {
		if tool.Name == name {
			return tool.Run(context.Background(), box, Arguments{Raw: json.RawMessage(args)})
		}
	}
	t.Fatalf("tool %q not found", name)
	return "", nil
}

// TestToolboxRejectsEscapingPaths is the guard that keeps a prompt-injected or
// confused model from writing outside the project it was pointed at.
func TestToolboxRejectsEscapingPaths(t *testing.T) {
	dir := t.TempDir()
	box, err := NewToolbox(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		"../escaped.txt",
		"../../etc/hosts",
		"sub/../../outside.txt",
		"/etc/passwd",
	} {
		if _, err := runTool(t, box, "write_file", `{"path":`+quote(path)+`,"content":"x"}`); err == nil {
			t.Errorf("write_file(%q) was allowed", path)
		}
		if _, err := runTool(t, box, "read_file", `{"path":`+quote(path)+`}`); err == nil {
			t.Errorf("read_file(%q) was allowed", path)
		}
	}

	// A path that stays inside is fine, including one that walks up and back.
	if _, err := runTool(t, box, "write_file", `{"path":"sub/../inside.txt","content":"ok"}`); err != nil {
		t.Errorf("in-project path rejected: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "inside.txt")); err != nil {
		t.Errorf("in-project write did not land: %v", err)
	}
}

// TestEditFileRequiresUniqueMatch: silently editing the first of several
// matches is how an agent corrupts a file.
func TestEditFileRequiresUniqueMatch(t *testing.T) {
	dir := t.TempDir()
	box, _ := NewToolbox(dir)
	path := filepath.Join(dir, "dup.go")
	if err := os.WriteFile(path, []byte("a := 1\na := 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := runTool(t, box, "edit_file", `{"path":"dup.go","old_string":"a := 1","new_string":"a := 2"}`); err == nil {
		t.Fatal("ambiguous edit was allowed")
	} else if !strings.Contains(err.Error(), "appears 2 times") {
		t.Errorf("unhelpful error: %v", err)
	}

	if _, err := runTool(t, box, "edit_file", `{"path":"dup.go","old_string":"a := 1","new_string":"a := 2","replace_all":true}`); err != nil {
		t.Fatalf("replace_all edit failed: %v", err)
	}
	content, _ := os.ReadFile(path)
	if string(content) != "a := 2\na := 2\n" {
		t.Errorf("content = %q", content)
	}
}

// TestEditFileMissingStringIsActionable: the model needs to know to re-read.
func TestEditFileMissingStringIsActionable(t *testing.T) {
	dir := t.TempDir()
	box, _ := NewToolbox(dir)
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runTool(t, box, "edit_file", `{"path":"x.go","old_string":"package y","new_string":"package z"}`)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want a not-found message", err)
	}
}

// TestRunCommandAllowlist keeps the agent away from arbitrary execution.
func TestRunCommandAllowlist(t *testing.T) {
	dir := t.TempDir()
	box, _ := NewToolbox(dir)

	for _, command := range []string{"rm -rf /", "curl http://example.com", "bash -c ls", "sudo reboot"} {
		if _, err := runTool(t, box, "run_command", `{"command":`+quote(command)+`}`); err == nil {
			t.Errorf("run_command(%q) was allowed", command)
		}
	}
	// Shell operators cannot work (no shell is used), so they must be refused
	// rather than passed through as literal arguments.
	if _, err := runTool(t, box, "run_command", `{"command":"ls ; rm -rf ."}`); err == nil {
		t.Error("shell operators were allowed")
	}
	if _, err := runTool(t, box, "run_command", `{"command":"go version"}`); err != nil {
		t.Errorf("allowlisted command rejected: %v", err)
	}

	box.AllowAnyCommand = true
	if _, err := runTool(t, box, "run_command", `{"command":"echo hi"}`); err != nil {
		t.Errorf("allow-any-command still refused: %v", err)
	}
}

// TestFailingCommandIsContent: a build error is information for the model, not
// a tool failure that should end the run.
func TestFailingCommandIsContent(t *testing.T) {
	dir := t.TempDir()
	box, _ := NewToolbox(dir)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module broken\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() { undefinedCall() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := runTool(t, box, "run_command", `{"command":"go build ./..."}`)
	if err != nil {
		t.Fatalf("failing build returned a tool error instead of output: %v", err)
	}
	if !strings.Contains(output, "command failed") || !strings.Contains(output, "undefinedCall") {
		t.Errorf("build failure not surfaced to the model: %q", output)
	}
}

// TestReadFileNumbersLines so edit_file can be aimed at an exact region.
func TestReadFileNumbersLinesAndPaginates(t *testing.T) {
	dir := t.TempDir()
	box, _ := NewToolbox(dir)
	var builder strings.Builder
	for i := 1; i <= 50; i++ {
		builder.WriteString("line\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), []byte(builder.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := runTool(t, box, "read_file", `{"path":"big.txt","offset":10,"limit":3}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output, "10\tline") {
		t.Errorf("output does not start at the offset: %q", output)
	}
	if strings.Count(output, "\tline") != 3 {
		t.Errorf("limit not honoured: %q", output)
	}
}

func quote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
