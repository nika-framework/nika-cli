package aiagent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFalseCompletionIsContradicted covers the failure mode weak models fall
// into most often: calling one read-only tool, then reporting the edit as done.
// Reporting that back unqualified would leave the user believing the change
// landed.
func TestFalseCompletionIsContradicted(t *testing.T) {
	agent, _ := newTestAgent(t, t.TempDir(), []stubTurn{
		{ToolCalls: []ToolCall{call("1", "project_info", `{}`)}},
		{Content: "Added a Phone string field to the User model and verified the changes."},
	})

	answer, err := agent.Run(context.Background(), "add a phone field", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(answer, "Nothing was actually changed") {
		t.Errorf("a false completion claim was passed through unqualified:\n%s", answer)
	}
}

// TestGenuineCompletionIsNotContradicted: the warning must not fire when the
// work really happened.
func TestGenuineCompletionIsNotContradicted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agent, _ := newTestAgent(t, dir, []stubTurn{
		{ToolCalls: []ToolCall{call("1", "write_file", `{"path":"a.txt","content":"new\n"}`)}},
		{Content: "Updated a.txt."},
	})

	answer, err := agent.Run(context.Background(), "rewrite the file", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(answer, "Nothing was actually changed") {
		t.Errorf("a real edit was flagged as a false claim:\n%s", answer)
	}
}

// TestAnswersWithoutClaimsAreLeftAlone: a question answered with no edits is
// not a false claim, and must not be decorated with a warning.
func TestAnswersWithoutClaimsAreLeftAlone(t *testing.T) {
	agent, _ := newTestAgent(t, t.TempDir(), []stubTurn{
		{ToolCalls: []ToolCall{call("1", "project_info", `{}`)}},
		{Content: "This project has three microservices: api, micro-grpc and micro-tcp."},
	})

	answer, err := agent.Run(context.Background(), "how many services?", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(answer, "Nothing was actually changed") {
		t.Errorf("an informational answer was flagged:\n%s", answer)
	}
}

// TestPresentTenseClaimIsCaught: models phrase completion loosely, and
// "these changes add a price field" is as misleading as "added a price field".
func TestPresentTenseClaimIsCaught(t *testing.T) {
	for name, answer := range map[string]string{
		"present tense":  "These changes add a required phone field with validation to the model.",
		"narrated diff":  "1. First, update the model:\n\n```go\n<<<<<<< SEARCH\ntype User struct {\n=======\ntype User struct {\n\tPhone string\n>>>>>>> REPLACE\n```\nThen rebuild the project to confirm everything still works correctly.",
		"gerund":         "Adding the Phone field to the User model and updating the create DTO accordingly.",
		"claims success": "Successfully generated the category module with all required layers.",
	} {
		t.Run(name, func(t *testing.T) {
			agent, _ := newTestAgent(t, t.TempDir(), []stubTurn{
				{ToolCalls: []ToolCall{call("1", "project_info", `{}`)}},
				{Content: answer},
			})
			got, err := agent.Run(context.Background(), "do the thing", nil)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if !strings.Contains(got, "Nothing was actually changed") {
				t.Errorf("claim not caught:\n%s", got)
			}
		})
	}
}

// TestPersianClaimIsCaught — the CLI is used in Persian, and a claim in
// Persian is just as misleading as one in English.
func TestPersianClaimIsCaught(t *testing.T) {
	agent, _ := newTestAgent(t, t.TempDir(), []stubTurn{
		{ToolCalls: []ToolCall{call("1", "project_info", `{}`)}},
		{Content: "فیلد قیمت به مدل محصول اضافه شد."},
	})

	answer, err := agent.Run(context.Background(), "به مدل محصول فیلد قیمت اضافه کن", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(answer, "Nothing was actually changed") {
		t.Errorf("Persian completion claim not caught:\n%s", answer)
	}
}
