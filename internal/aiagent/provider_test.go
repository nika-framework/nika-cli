package aiagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nika-framework/nika-cli/internal/nikaconf"
)

// TestOllamaProviderRoundTrip covers the default provider's wire format, which
// differs from OpenAI's in two ways that break the loop if mishandled: the
// reply is a single "message" object rather than a choices array, and tool
// calls carry arguments as a nested object with no call ID.
func TestOllamaProviderRoundTrip(t *testing.T) {
	var gotPath string
	var gotRequest ollamaRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Errorf("decode: %v", err)
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[
			{"function":{"name":"read_file","arguments":{"path":"go.mod"}}}
		]}}`))
	}))
	defer server.Close()

	provider, err := NewProvider(nikaconf.AgentConfig{Provider: "ollama", Model: "qwen2.5-coder:7b", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	provider.Client = server.Client()

	reply, err := provider.Chat(context.Background(),
		[]Message{{Role: RoleUser, Content: "read go.mod"}},
		Schemas(Tools()))
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if gotPath != "/api/chat" {
		t.Errorf("path = %q, want /api/chat", gotPath)
	}
	if gotRequest.Stream {
		t.Error("stream must be false: the loop reads one whole reply at a time")
	}
	if len(gotRequest.Tools) == 0 {
		t.Error("tools were not advertised to the model")
	}
	if len(reply.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(reply.ToolCalls))
	}
	// Ollama omits IDs; the loop needs one to pair the result with the call.
	if reply.ToolCalls[0].ID == "" {
		t.Error("tool call ID was not synthesised")
	}
	var args struct {
		Path string `json:"path"`
	}
	if err := reply.ToolCalls[0].Function.Arguments.Decode(&args); err != nil {
		t.Fatalf("decode arguments: %v", err)
	}
	if args.Path != "go.mod" {
		t.Errorf("path = %q", args.Path)
	}
}

// TestAnthropicProviderMapsToolResults: Anthropic pairs tool results with the
// requesting turn by ID and takes the system prompt out of band, so the
// generic message list has to be reshaped rather than sent as-is.
func TestAnthropicProviderMapsToolResults(t *testing.T) {
	var got anthropicRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key := r.Header.Get("x-api-key"); key != "secret" {
			t.Errorf("x-api-key = %q", key)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode: %v", err)
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"done"}]}`))
	}))
	defer server.Close()

	t.Setenv("TEST_ANTHROPIC_KEY", "secret")
	provider, err := NewProvider(nikaconf.AgentConfig{
		Provider: "claude", Model: "claude-sonnet-4-5", BaseURL: server.URL, APIKeyEnv: "TEST_ANTHROPIC_KEY",
	})
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	provider.Client = server.Client()

	reply, err := provider.Chat(context.Background(), []Message{
		{Role: RoleSystem, Content: "you are the nika agent"},
		{Role: RoleUser, Content: "read go.mod"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{call("tool_1", "read_file", `{"path":"go.mod"}`)}},
		{Role: RoleTool, ToolCallID: "tool_1", Content: "module example.com/app"},
	}, Schemas(Tools()))
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if reply.Content != "done" {
		t.Errorf("content = %q", reply.Content)
	}

	if !strings.Contains(got.System, "nika agent") {
		t.Errorf("system prompt not hoisted out of the message list: %q", got.System)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(got.Messages))
	}
	assistant := got.Messages[1]
	if assistant.Role != "assistant" || len(assistant.Content) != 1 || assistant.Content[0].Type != "tool_use" {
		t.Fatalf("assistant turn not mapped to tool_use: %+v", assistant)
	}
	if assistant.Content[0].ID != "tool_1" {
		t.Errorf("tool_use id = %q", assistant.Content[0].ID)
	}
	result := got.Messages[2]
	if result.Role != "user" || result.Content[0].Type != "tool_result" || result.Content[0].ToolUseID != "tool_1" {
		t.Errorf("tool result not paired by id: %+v", result)
	}
	if len(got.Tools) == 0 || got.Tools[0].InputSchema == nil {
		t.Error("tools were not translated to Anthropic's input_schema shape")
	}
}

// TestProviderRejectsIncompleteConfig keeps misconfiguration from surfacing as
// a confusing HTTP error later.
func TestProviderRejectsIncompleteConfig(t *testing.T) {
	for name, config := range map[string]nikaconf.AgentConfig{
		"no provider": {Model: "x", BaseURL: "http://localhost"},
		"unknown":     {Provider: "gemini", Model: "x", BaseURL: "http://localhost"},
		"no model":    {Provider: "chatgpt", BaseURL: "http://localhost"},
		"no base url": {Provider: "chatgpt", Model: "gpt-4o-mini"},
	} {
		if _, err := NewProvider(config); err == nil {
			t.Errorf("%s: NewProvider() succeeded", name)
		}
	}

	// Ollama is the exception: it has a well-known local default.
	provider, err := NewProvider(nikaconf.AgentConfig{Provider: "ollama", Model: "qwen2.5-coder:7b"})
	if err != nil {
		t.Fatalf("ollama without base_url: %v", err)
	}
	if provider.BaseURL != "http://localhost:11434" {
		t.Errorf("default base url = %q", provider.BaseURL)
	}
}

// TestMissingAPIKeyIsNamed so the user knows which variable to export.
func TestMissingAPIKeyIsNamed(t *testing.T) {
	t.Setenv("TEST_MISSING_KEY", "")
	provider, err := NewProvider(nikaconf.AgentConfig{
		Provider: "chatgpt", Model: "gpt-4o-mini", BaseURL: "http://localhost:1", APIKeyEnv: "TEST_MISSING_KEY",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "TEST_MISSING_KEY") {
		t.Errorf("err = %v, want it to name the missing variable", err)
	}
}
