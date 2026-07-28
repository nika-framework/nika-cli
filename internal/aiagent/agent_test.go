package aiagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubTurn is one canned assistant reply from the fake provider.
type stubTurn struct {
	Content   string
	ToolCalls []ToolCall
}

// newStubProvider stands up an OpenAI-compatible endpoint that replays turns in
// order, and records what the agent sent. It exercises the real HTTP encoding
// path rather than swapping the provider out for an interface.
func newStubProvider(t *testing.T, turns []stubTurn) (*Provider, *[]openAIRequest) {
	t.Helper()
	var received []openAIRequest
	index := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request openAIRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode agent request: %v", err)
		}
		received = append(received, request)

		if index >= len(turns) {
			t.Errorf("provider called %d times, only %d turns scripted", index+1, len(turns))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		turn := turns[index]
		index++

		reply := openAIReply{}
		reply.Choices = append(reply.Choices, struct {
			Message Message `json:"message"`
		}{Message: Message{Role: RoleAssistant, Content: turn.Content, ToolCalls: turn.ToolCalls}})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(reply)
	}))
	t.Cleanup(server.Close)

	return &Provider{
		Name:    "chatgpt",
		Model:   "stub",
		BaseURL: server.URL,
		Client:  server.Client(),
	}, &received
}

// newTestAgent wires a stub provider to a real toolbox over dir.
func newTestAgent(t *testing.T, dir string, turns []stubTurn) (*Agent, *[]openAIRequest) {
	t.Helper()
	provider, received := newStubProvider(t, turns)
	box, err := NewToolbox(dir)
	if err != nil {
		t.Fatalf("NewToolbox() error = %v", err)
	}
	agent := &Agent{Provider: provider, Toolbox: box, MaxSteps: 10, tools: map[string]Tool{}}
	toolset := Tools()
	for _, tool := range toolset {
		agent.tools[tool.Name] = tool
	}
	agent.schemas = Schemas(toolset)
	agent.messages = []Message{{Role: RoleSystem, Content: "test system prompt"}}
	return agent, received
}

func call(id, name, args string) ToolCall {
	return ToolCall{ID: id, Type: "function", Function: FunctionCall{Name: name, Arguments: Arguments{Raw: json.RawMessage(args)}}}
}

// TestAgentLoopEditsAFile is the behaviour the keyword dispatcher could not do:
// an arbitrary instruction turning into read → edit → answer, with the file
// actually changed on disk.
func TestAgentLoopEditsAFile(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "product.model.go")
	original := "package schema\n\ntype Product struct {\n\tName string `db:\"name\"`\n}\n"
	if err := os.WriteFile(model, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	agent, received := newTestAgent(t, dir, []stubTurn{
		{ToolCalls: []ToolCall{call("1", "read_file", `{"path":"product.model.go"}`)}},
		{ToolCalls: []ToolCall{call("2", "edit_file", `{"path":"product.model.go","old_string":"\tName string `+"`"+`db:\"name\"`+"`"+`\n","new_string":"\tName string `+"`"+`db:\"name\"`+"`"+`\n\tPrice float64 `+"`"+`db:\"price\"`+"`"+`\n"}`)}},
		{Content: "Added the price field."},
	})

	var kinds []EventKind
	answer, err := agent.Run(context.Background(), "add a price field", func(event Event) {
		kinds = append(kinds, event.Kind)
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if answer != "Added the price field." {
		t.Errorf("answer = %q", answer)
	}

	updated, err := os.ReadFile(model)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "Price float64") {
		t.Errorf("model was not edited:\n%s", updated)
	}

	// The tool result must be fed back, otherwise the model is editing blind.
	if len(*received) != 3 {
		t.Fatalf("provider calls = %d, want 3", len(*received))
	}
	second := (*received)[1]
	last := second.Messages[len(second.Messages)-1]
	if last.Role != RoleTool || last.ToolCallID != "1" {
		t.Errorf("second request did not end with the read_file result: %+v", last)
	}
	if !strings.Contains(last.Content, "type Product struct") {
		t.Errorf("read_file result not passed back: %q", last.Content)
	}

	if changed := agent.ChangedFiles(); len(changed) != 1 || changed[0] != "product.model.go" {
		t.Errorf("changed files = %v", changed)
	}
	if !containsKind(kinds, EventToolCall) || !containsKind(kinds, EventDone) {
		t.Errorf("event kinds = %v", kinds)
	}
}

// TestAgentRecoversFromToolError checks that a bad tool call is handed back as
// content instead of aborting: the model must get a chance to fix it.
func TestAgentRecoversFromToolError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	agent, received := newTestAgent(t, dir, []stubTurn{
		{ToolCalls: []ToolCall{call("1", "read_file", `{"path":"missing.txt"}`)}},
		{ToolCalls: []ToolCall{call("2", "read_file", `{"path":"a.txt"}`)}},
		{Content: "The file says hello."},
	})

	if _, err := agent.Run(context.Background(), "read the file", nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(*received) != 3 {
		t.Fatalf("provider calls = %d, want 3 (the run must survive a failed tool)", len(*received))
	}
	second := (*received)[1]
	last := second.Messages[len(second.Messages)-1]
	if !strings.Contains(last.Content, "error") {
		t.Errorf("failed tool result not returned as content: %q", last.Content)
	}
}

// TestAgentStopsAtMaxSteps guards against a model that loops forever.
func TestAgentStopsAtMaxSteps(t *testing.T) {
	dir := t.TempDir()
	turns := make([]stubTurn, 4)
	for i := range turns {
		turns[i] = stubTurn{ToolCalls: []ToolCall{call("x", "list_dir", `{"path":"."}`)}}
	}
	agent, _ := newTestAgent(t, dir, turns)
	agent.MaxSteps = 3

	if _, err := agent.Run(context.Background(), "loop forever", nil); err == nil {
		t.Fatal("Run() succeeded, want a max-steps error")
	}
}

// TestUnknownToolIsReported keeps a hallucinated tool name from killing a run.
func TestUnknownToolIsReported(t *testing.T) {
	agent, received := newTestAgent(t, t.TempDir(), []stubTurn{
		{ToolCalls: []ToolCall{call("1", "delete_everything", `{}`)}},
		{Content: "Sorry, I cannot do that."},
	})
	if _, err := agent.Run(context.Background(), "delete it all", nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	second := (*received)[1]
	last := second.Messages[len(second.Messages)-1]
	if !strings.Contains(last.Content, "unknown tool") {
		t.Errorf("unknown tool not reported back: %q", last.Content)
	}
}

// TestReadOnlyBlocksMutation verifies --read-only actually protects the tree.
func TestReadOnlyBlocksMutation(t *testing.T) {
	dir := t.TempDir()
	agent, _ := newTestAgent(t, dir, []stubTurn{
		{ToolCalls: []ToolCall{call("1", "write_file", `{"path":"new.txt","content":"x"}`)}},
		{Content: "I could not write the file."},
	})
	agent.Toolbox.ReadOnly = true

	if _, err := agent.Run(context.Background(), "write a file", nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); !os.IsNotExist(err) {
		t.Error("read-only session wrote a file")
	}
}

// TestArgumentsAcceptBothEncodings covers the two shapes providers actually
// send: OpenAI stringifies the arguments object, Ollama nests it.
func TestArgumentsAcceptBothEncodings(t *testing.T) {
	var stringified struct {
		Function FunctionCall `json:"function"`
	}
	if err := json.Unmarshal([]byte(`{"function":{"name":"read_file","arguments":"{\"path\":\"a.go\"}"}}`), &stringified); err != nil {
		t.Fatalf("stringified arguments: %v", err)
	}
	var nested struct {
		Function FunctionCall `json:"function"`
	}
	if err := json.Unmarshal([]byte(`{"function":{"name":"read_file","arguments":{"path":"a.go"}}}`), &nested); err != nil {
		t.Fatalf("nested arguments: %v", err)
	}

	for name, args := range map[string]Arguments{
		"stringified": stringified.Function.Arguments,
		"nested":      nested.Function.Arguments,
	} {
		var decoded struct {
			Path string `json:"path"`
		}
		if err := args.Decode(&decoded); err != nil {
			t.Fatalf("%s decode: %v", name, err)
		}
		if decoded.Path != "a.go" {
			t.Errorf("%s path = %q, want a.go", name, decoded.Path)
		}
	}
}

func containsKind(kinds []EventKind, want EventKind) bool {
	for _, kind := range kinds {
		if kind == want {
			return true
		}
	}
	return false
}
