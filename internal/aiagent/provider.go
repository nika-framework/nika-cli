// Package aiagent turns the configured LLM into an agent that can actually
// change a project.
//
// The previous AI path matched keywords ("module", "بساز") and dispatched to
// one of two fixed generators, so anything outside those two shapes — "add a
// price field to the product model", "rename this route" — was answered with
// prose and no edits. This package replaces that with a tool-calling loop: the
// model is given file and command tools plus Nika's own generator, and it runs
// until the task is done.
package aiagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nika-framework/nika-cli/internal/nikaconf"
)

// Role values used in the conversation.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Message is one turn in the conversation, in the OpenAI wire shape. Ollama's
// /api/chat accepts the same fields, which is why one struct serves both.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall is a function invocation requested by the model.
type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function FunctionCall `json:"function"`
}

// FunctionCall names the tool and carries its JSON arguments.
type FunctionCall struct {
	Name string `json:"name"`
	// Arguments is a JSON object. OpenAI sends it as a string, Ollama as an
	// object, so it is decoded leniently by UnmarshalJSON.
	Arguments Arguments `json:"arguments"`
}

// Arguments holds tool arguments as raw JSON, tolerating both encodings.
type Arguments struct {
	Raw json.RawMessage
}

// UnmarshalJSON accepts either a JSON object or a JSON string containing one.
func (a *Arguments) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		a.Raw = json.RawMessage("{}")
		return nil
	}
	if trimmed[0] == '"' {
		var encoded string
		if err := json.Unmarshal(trimmed, &encoded); err != nil {
			return err
		}
		if strings.TrimSpace(encoded) == "" {
			encoded = "{}"
		}
		a.Raw = json.RawMessage(encoded)
		return nil
	}
	a.Raw = append(json.RawMessage(nil), trimmed...)
	return nil
}

// MarshalJSON re-emits the raw object.
func (a Arguments) MarshalJSON() ([]byte, error) {
	if len(a.Raw) == 0 {
		return []byte("{}"), nil
	}
	return a.Raw, nil
}

// Decode unmarshals the arguments into target.
func (a Arguments) Decode(target any) error {
	raw := a.Raw
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	return json.Unmarshal(raw, target)
}

// String renders the arguments for display.
func (a Arguments) String() string {
	if len(a.Raw) == 0 {
		return "{}"
	}
	return string(a.Raw)
}

// ToolSchema advertises one callable tool to the model.
type ToolSchema struct {
	Type     string         `json:"type"`
	Function FunctionSchema `json:"function"`
}

// FunctionSchema is the JSON Schema description of a tool.
type FunctionSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolMode is how tool calls reach the model.
type ToolMode int

const (
	// ToolsAuto probes the backend and decides on first use.
	ToolsAuto ToolMode = iota
	// ToolsNative uses the provider's function-calling API.
	ToolsNative
	// ToolsEmulated describes the tools in the prompt and parses JSON replies.
	ToolsEmulated
)

// Provider talks to one LLM backend.
type Provider struct {
	Name      string
	Model     string
	BaseURL   string
	APIKeyEnv string
	Client    *http.Client

	// Tools selects native or emulated function calling.
	Tools ToolMode
	// turn counts replies, so emulated tool calls get distinct IDs.
	turn int
}

// NewProvider builds a provider from the [agent] section of .nika.toml.
func NewProvider(config nikaconf.AgentConfig) (*Provider, error) {
	name := strings.ToLower(strings.TrimSpace(config.Provider))
	switch name {
	case "ollama", "ollema":
		name = "ollama"
	case "9router", "openrouter":
		name = "9router"
	case "chatgpt", "openai":
		name = "chatgpt"
	case "claude", "anthropic":
		name = "claude"
	case "":
		return nil, fmt.Errorf("no AI provider configured — run `nika agent init <ollama|chatgpt|9router|claude>`")
	default:
		return nil, fmt.Errorf("unknown agent provider %q", config.Provider)
	}

	model := strings.TrimSpace(config.Model)
	if model == "" {
		return nil, fmt.Errorf("agent model is empty in %s", nikaconf.FileName)
	}
	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" && name == "ollama" {
		baseURL = "http://localhost:11434"
	}
	if baseURL == "" {
		return nil, fmt.Errorf("agent base_url is empty in %s", nikaconf.FileName)
	}
	if !strings.Contains(baseURL, "://") {
		baseURL = "http://" + baseURL
	}

	return &Provider{
		Name:      name,
		Model:     model,
		BaseURL:   strings.TrimRight(baseURL, "/"),
		APIKeyEnv: strings.TrimSpace(config.APIKeyEnv),
		// Long timeout: a coding model reasoning over a large file can take
		// minutes, and a truncated request looks to the user like a hang.
		Client: &http.Client{Timeout: 10 * time.Minute},
	}, nil
}

// Describe is a short provider label for the UI.
func (p *Provider) Describe() string {
	label := p.Name + " / " + p.Model
	if p.Tools == ToolsEmulated {
		label += " (emulated tools)"
	}
	return label
}

// Chat sends the conversation and returns the assistant's next message.
//
// When the backend cannot call functions natively, the same request is served
// through the emulated protocol, so callers never have to care which mode is
// in use.
func (p *Provider) Chat(ctx context.Context, messages []Message, tools []ToolSchema) (Message, error) {
	p.turn++

	if len(tools) > 0 {
		if p.Tools == ToolsAuto {
			p.Tools = p.detectToolMode()
		}
		if p.Tools == ToolsEmulated {
			return p.chatEmulated(ctx, messages, tools)
		}
	}

	reply, err := p.chatNative(ctx, messages, tools)
	if err != nil && len(tools) > 0 && isUnsupportedToolsError(err) {
		// The probe said native was fine but the backend disagrees. Switch and
		// retry once rather than surfacing an error the user cannot act on.
		p.Tools = ToolsEmulated
		return p.chatEmulated(ctx, messages, tools)
	}
	if err == nil {
		// Reasoning models leak their scratchpad into the content field even on
		// the native path, where nothing else would strip it.
		reply.Content = strings.TrimSpace(thinkBlock.ReplaceAllString(reply.Content, ""))
	}
	return reply, err
}

func (p *Provider) chatNative(ctx context.Context, messages []Message, tools []ToolSchema) (Message, error) {
	switch p.Name {
	case "ollama":
		return p.chatOllama(ctx, messages, tools)
	case "claude":
		return p.chatAnthropic(ctx, messages, tools)
	default:
		return p.chatOpenAI(ctx, messages, tools)
	}
}

// chatEmulated runs one turn of the JSON tool protocol.
func (p *Provider) chatEmulated(ctx context.Context, messages []Message, tools []ToolSchema) (Message, error) {
	reply, err := p.chatNative(ctx, toEmulatedMessages(messages, tools), nil)
	if err != nil {
		return Message{}, err
	}
	return parseEmulatedReply(reply.Content, p.turn), nil
}

// detectToolMode asks the backend whether the model can call functions.
//
// Only Ollama is probed: it reports capabilities locally and cheaply, and it
// is the backend where a chat-only model is likely. Cloud providers are
// assumed native and fall back reactively if that turns out to be wrong.
func (p *Provider) detectToolMode() ToolMode {
	if p.Name != "ollama" {
		return ToolsNative
	}
	capabilities := ollamaCapabilities(p.Client, p.BaseURL, p.Model)
	if capabilities == nil {
		// Unknown — try native and fall back on the error.
		return ToolsNative
	}
	for _, capability := range capabilities {
		if capability == "tools" {
			return ToolsNative
		}
	}
	return ToolsEmulated
}

// isUnsupportedToolsError recognises a backend refusing function calling.
func isUnsupportedToolsError(err error) bool {
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"does not support tools",
		"does not support function",
		"tools is not supported",
		"tool use is not supported",
		"unsupported parameter: 'tools'",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func (p *Provider) apiKey() (string, error) {
	if p.APIKeyEnv == "" {
		return "", nil
	}
	key := strings.TrimSpace(os.Getenv(p.APIKeyEnv))
	if key == "" {
		return "", fmt.Errorf("environment variable %s is not set", p.APIKeyEnv)
	}
	return key, nil
}

// ── OpenAI-compatible (chatgpt, openrouter) ─────────────────────────

type openAIRequest struct {
	Model    string       `json:"model"`
	Messages []Message    `json:"messages"`
	Tools    []ToolSchema `json:"tools,omitempty"`
	Stream   bool         `json:"stream"`
}

type openAIReply struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *Provider) chatOpenAI(ctx context.Context, messages []Message, tools []ToolSchema) (Message, error) {
	key, err := p.apiKey()
	if err != nil {
		return Message{}, err
	}
	endpoint := p.BaseURL
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}
	body, err := json.Marshal(openAIRequest{Model: p.Model, Messages: messages, Tools: tools})
	if err != nil {
		return Message{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	raw, err := p.do(req)
	if err != nil {
		return Message{}, err
	}
	var reply openAIReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return Message{}, fmt.Errorf("decode %s response: %w", p.Name, err)
	}
	if reply.Error.Message != "" {
		return Message{}, fmt.Errorf("%s: %s", p.Name, reply.Error.Message)
	}
	if len(reply.Choices) == 0 {
		return Message{}, fmt.Errorf("%s returned no choices", p.Name)
	}
	out := reply.Choices[0].Message
	out.Role = RoleAssistant
	return out, nil
}

// ── Ollama ──────────────────────────────────────────────────────────

type ollamaRequest struct {
	Model    string       `json:"model"`
	Messages []Message    `json:"messages"`
	Tools    []ToolSchema `json:"tools,omitempty"`
	Stream   bool         `json:"stream"`
}

type ollamaReply struct {
	Message Message `json:"message"`
	Error   string  `json:"error"`
}

func (p *Provider) chatOllama(ctx context.Context, messages []Message, tools []ToolSchema) (Message, error) {
	body, err := json.Marshal(ollamaRequest{Model: p.Model, Messages: messages, Tools: tools})
	if err != nil {
		return Message{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	raw, err := p.do(req)
	if err != nil {
		// Leave "does not support tools" alone: Chat catches it and retries in
		// emulated mode, so it must stay recognisable.
		if strings.Contains(err.Error(), "connect to") {
			return Message{}, fmt.Errorf("%w — is Ollama running at %s?", err, p.BaseURL)
		}
		if strings.Contains(err.Error(), "not found, try pulling") {
			return Message{}, fmt.Errorf("the Ollama model %q is not installed.\n"+
				"Install it with `ollama pull %s`, or run `nika agent init ollama` "+
				"to pick one of the models you already have", p.Model, p.Model)
		}
		return Message{}, err
	}
	var reply ollamaReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return Message{}, fmt.Errorf("decode Ollama response: %w", err)
	}
	if reply.Error != "" {
		return Message{}, fmt.Errorf("Ollama: %s", reply.Error)
	}
	out := reply.Message
	out.Role = RoleAssistant
	// Ollama omits call IDs; the loop needs them to pair results with calls.
	for i := range out.ToolCalls {
		if out.ToolCalls[i].ID == "" {
			out.ToolCalls[i].ID = fmt.Sprintf("call_%d", i+1)
		}
	}
	return out, nil
}

// ── Anthropic ───────────────────────────────────────────────────────

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicReply struct {
	Content []anthropicContent `json:"content"`
	Error   struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *Provider) chatAnthropic(ctx context.Context, messages []Message, tools []ToolSchema) (Message, error) {
	key, err := p.apiKey()
	if err != nil {
		return Message{}, err
	}

	request := anthropicRequest{Model: p.Model, MaxTokens: 8192}
	for _, tool := range tools {
		request.Tools = append(request.Tools, anthropicTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.Parameters,
		})
	}
	// Anthropic takes the system prompt out of band, and pairs tool results
	// with the assistant turn that requested them by ID.
	for _, message := range messages {
		switch message.Role {
		case RoleSystem:
			if request.System != "" {
				request.System += "\n\n"
			}
			request.System += message.Content
		case RoleTool:
			request.Messages = append(request.Messages, anthropicMessage{
				Role: "user",
				Content: []anthropicContent{{
					Type:      "tool_result",
					ToolUseID: message.ToolCallID,
					Content:   message.Content,
				}},
			})
		case RoleAssistant:
			var content []anthropicContent
			if strings.TrimSpace(message.Content) != "" {
				content = append(content, anthropicContent{Type: "text", Text: message.Content})
			}
			for _, call := range message.ToolCalls {
				content = append(content, anthropicContent{
					Type:  "tool_use",
					ID:    call.ID,
					Name:  call.Function.Name,
					Input: json.RawMessage(call.Function.Arguments.String()),
				})
			}
			if len(content) == 0 {
				continue
			}
			request.Messages = append(request.Messages, anthropicMessage{Role: "assistant", Content: content})
		default:
			request.Messages = append(request.Messages, anthropicMessage{
				Role:    "user",
				Content: []anthropicContent{{Type: "text", Text: message.Content}},
			})
		}
	}

	body, err := json.Marshal(request)
	if err != nil {
		return Message{}, err
	}
	endpoint := p.BaseURL
	if !strings.HasSuffix(endpoint, "/messages") {
		endpoint += "/messages"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")

	raw, err := p.do(req)
	if err != nil {
		return Message{}, err
	}
	var reply anthropicReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return Message{}, fmt.Errorf("decode Anthropic response: %w", err)
	}
	if reply.Error.Message != "" {
		return Message{}, fmt.Errorf("anthropic: %s", reply.Error.Message)
	}

	out := Message{Role: RoleAssistant}
	for _, block := range reply.Content {
		switch block.Type {
		case "text":
			out.Content += block.Text
		case "tool_use":
			out.ToolCalls = append(out.ToolCalls, ToolCall{
				ID:       block.ID,
				Type:     "function",
				Function: FunctionCall{Name: block.Name, Arguments: Arguments{Raw: block.Input}},
			})
		}
	}
	return out, nil
}

// do performs the request and surfaces the body on non-2xx, because provider
// errors are usually only readable there.
func (p *Provider) do(req *http.Request) ([]byte, error) {
	res, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", p.Name, err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", p.Name, err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(raw))
		if len(snippet) > 500 {
			snippet = snippet[:500] + "…"
		}
		return nil, fmt.Errorf("%s returned HTTP %s: %s", p.Name, res.Status, snippet)
	}
	return raw, nil
}
