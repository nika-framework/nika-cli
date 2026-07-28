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

	"github.com/nika-framework/nika-cli/internal/nikaconf"
)

func TestParseEmulatedReply(t *testing.T) {
	cases := map[string]struct {
		reply    string
		wantTool string
		wantArg  string
		wantText string
	}{
		"bare object": {
			reply:    `{"tool":"read_file","arguments":{"path":"go.mod"}}`,
			wantTool: "read_file", wantArg: "go.mod",
		},
		"fenced": {
			reply:    "```json\n{\"tool\":\"read_file\",\"arguments\":{\"path\":\"go.mod\"}}\n```",
			wantTool: "read_file", wantArg: "go.mod",
		},
		"with surrounding prose": {
			reply:    "Sure, let me look.\n\n{\"tool\":\"read_file\",\"arguments\":{\"path\":\"go.mod\"}}\n\nThat should do it.",
			wantTool: "read_file", wantArg: "go.mod",
		},
		"reasoning block stripped": {
			reply:    "<think>The user wants the file.\nI should read it.</think>\n{\"tool\":\"read_file\",\"arguments\":{\"path\":\"go.mod\"}}",
			wantTool: "read_file", wantArg: "go.mod",
		},
		"openai vocabulary": {
			reply:    `{"name":"read_file","args":{"path":"go.mod"}}`,
			wantTool: "read_file", wantArg: "go.mod",
		},
		"final answer": {
			reply:    `{"answer":"Added the price field."}`,
			wantText: "Added the price field.",
		},
		"plain prose": {
			reply:    "Dependency injection wires components together.",
			wantText: "Dependency injection wires components together.",
		},
		"braces inside a string do not confuse the scan": {
			reply:    `{"tool":"write_file","arguments":{"path":"a.go","content":"func main() { }"}}`,
			wantTool: "write_file", wantArg: "a.go",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			message := parseEmulatedReply(testCase.reply, 1)

			if testCase.wantTool == "" {
				if len(message.ToolCalls) != 0 {
					t.Fatalf("unexpected tool call: %+v", message.ToolCalls)
				}
				if strings.TrimSpace(message.Content) != testCase.wantText {
					t.Errorf("content = %q, want %q", message.Content, testCase.wantText)
				}
				return
			}

			if len(message.ToolCalls) != 1 {
				t.Fatalf("tool calls = %d, want 1 (content %q)", len(message.ToolCalls), message.Content)
			}
			call := message.ToolCalls[0]
			if call.Function.Name != testCase.wantTool {
				t.Errorf("tool = %q, want %q", call.Function.Name, testCase.wantTool)
			}
			if call.ID == "" {
				t.Error("tool call has no ID")
			}
			var args struct {
				Path string `json:"path"`
			}
			if err := call.Function.Arguments.Decode(&args); err != nil {
				t.Fatalf("decode arguments: %v", err)
			}
			if args.Path != testCase.wantArg {
				t.Errorf("path = %q, want %q", args.Path, testCase.wantArg)
			}
		})
	}
}

// TestToEmulatedMessages checks the transcript stays consistent with the
// protocol the model is being asked to follow: no tool role, and its own past
// calls replayed as the JSON it was told to emit.
func TestToEmulatedMessages(t *testing.T) {
	converted := toEmulatedMessages([]Message{
		{Role: RoleSystem, Content: "you are the nika agent"},
		{Role: RoleUser, Content: "read go.mod"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{call("c1", "read_file", `{"path":"go.mod"}`)}},
		{Role: RoleTool, ToolCallID: "c1", Name: "read_file", Content: "module example.com/app"},
	}, Schemas(Tools()))

	for _, message := range converted {
		if message.Role == RoleTool {
			t.Error("a tool-role message survived conversion")
		}
		if len(message.ToolCalls) > 0 {
			t.Error("a native tool call survived conversion")
		}
	}
	if !strings.Contains(converted[0].Content, "you are the nika agent") {
		t.Error("original system prompt was dropped")
	}
	if !strings.Contains(converted[0].Content, `{"tool": "<tool name>"`) {
		t.Error("protocol instructions were not appended to the system prompt")
	}
	if !strings.Contains(converted[0].Content, "## read_file") {
		t.Error("tool catalogue missing from the prompt")
	}

	replay := converted[2]
	if replay.Role != RoleAssistant || !strings.Contains(replay.Content, `"read_file"`) {
		t.Errorf("assistant tool call not replayed as JSON: %+v", replay)
	}
	result := converted[3]
	if result.Role != RoleUser || !strings.Contains(result.Content, "module example.com/app") {
		t.Errorf("tool result not delivered as a user turn: %+v", result)
	}
}

// TestEmulatedLoopEditsAFile is the whole point: a model with no function
// calling at all still completes a real task.
func TestEmulatedLoopEditsAFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "user.model.go")
	if err := os.WriteFile(target, []byte("type User struct {\n\tName string\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A chat-only backend: it refuses any request carrying tools, and replies
	// with plain text otherwise.
	replies := []string{
		`{"tool":"read_file","arguments":{"path":"user.model.go"}}`,
		"```json\n{\"tool\":\"edit_file\",\"arguments\":{\"path\":\"user.model.go\",\"old_string\":\"\\tName string\",\"new_string\":\"\\tName string\\n\\tAge int\"}}\n```",
		`{"answer":"Added an Age field."}`,
	}
	index := 0
	var sawTools bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request ollamaRequest
		_ = json.NewDecoder(r.Body).Decode(&request)
		if len(request.Tools) > 0 {
			sawTools = true
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"registry.ollama.ai/library/gemma3:4b does not support tools"}`))
			return
		}
		reply := `{"message":{"role":"assistant","content":` + mustJSON(replies[index]) + `}}`
		index++
		_, _ = w.Write([]byte(reply))
	}))
	defer server.Close()

	provider, err := NewProvider(nikaconf.AgentConfig{Provider: "ollama", Model: "gemma3:4b", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	provider.Client = server.Client()
	// Force the reactive path: pretend the capability probe said "native".
	provider.Tools = ToolsNative

	box, err := NewToolbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	agent := &Agent{Provider: provider, Toolbox: box, MaxSteps: 10, tools: map[string]Tool{}}
	for _, tool := range Tools() {
		agent.tools[tool.Name] = tool
	}
	agent.schemas = Schemas(Tools())
	agent.messages = []Message{{Role: RoleSystem, Content: "test"}}

	answer, err := agent.Run(context.Background(), "add an Age field", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !sawTools {
		t.Error("the native attempt never happened")
	}
	if provider.Tools != ToolsEmulated {
		t.Error("provider did not switch to emulated mode")
	}
	if answer != "Added an Age field." {
		t.Errorf("answer = %q", answer)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Age int") {
		t.Errorf("file not edited through the emulated protocol:\n%s", content)
	}
}

// TestNudgeOnUnexecutedWork covers the reply shape weak models produce: a long
// narration of edits, with invented paths, and no tool call.
func TestNudgeOnUnexecutedWork(t *testing.T) {
	narration := "I'll help add the price field. This requires changes to multiple files.\n\n" +
		"1. First, let's update the product model:\n\n```go\n<<<<<<< SEARCH\ntype Product struct {\n" +
		"=======\ntype Product struct {\n    Price float64\n>>>>>>> REPLACE\n```\n" +
		"2. Now update the DTO similarly, and regenerate the repository interface."

	// After the nudge the provider switches to the explicit JSON protocol, so
	// the remaining turns come back as text rather than native tool calls.
	agent, received := newTestAgent(t, t.TempDir(), []stubTurn{
		{Content: narration},
		{Content: `{"tool":"project_info","arguments":{}}`},
		{Content: `{"answer":"Done."}`},
	})

	var statuses []string
	answer, err := agent.Run(context.Background(), "add a price field", func(event Event) {
		if event.Kind == EventStatus {
			statuses = append(statuses, event.Text)
		}
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if answer != "Done." {
		t.Errorf("answer = %q, want the post-nudge answer", answer)
	}
	if len(*received) != 3 {
		t.Fatalf("provider calls = %d, want 3 (narration, nudge, answer)", len(*received))
	}
	if agent.Provider.Tools != ToolsEmulated {
		t.Error("the nudge did not switch to the explicit JSON protocol")
	}

	// The nudge itself must reach the model, and the tools must now be
	// described in the prompt rather than sent as a tools array.
	second := (*received)[1]
	if len(second.Tools) != 0 {
		t.Error("still sending a native tools array after switching to emulation")
	}
	var sawNudge bool
	for _, message := range second.Messages {
		if strings.Contains(message.Content, "have not called any tool") {
			sawNudge = true
		}
	}
	if !sawNudge {
		t.Error("nudge text never reached the model")
	}

	var nudged bool
	for _, status := range statuses {
		if strings.Contains(status, "described the change instead") {
			nudged = true
		}
	}
	if !nudged {
		t.Errorf("nudge not surfaced to the user: %v", statuses)
	}
}

// TestNoNudgeForGenuineAnswers keeps question-answering from looping.
func TestNoNudgeForGenuineAnswers(t *testing.T) {
	for name, answer := range map[string]string{
		"short": "This project has three microservices: api, micro-grpc and micro-tcp.",
		"long prose without edit markers": strings.Repeat(
			"Dependency injection in Nika wires providers into controllers through the module graph. ", 5),
	} {
		t.Run(name, func(t *testing.T) {
			if looksLikeUnexecutedWork(answer) {
				t.Errorf("genuine answer flagged as unexecuted work: %q", answer)
			}
		})
	}
}

func mustJSON(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
