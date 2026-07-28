package internal

import "testing"

func TestDefaultAgentConfig(t *testing.T) {
	// The default must be a tool-calling model: the agent loop can only edit a
	// project through function calls, so a chat-only default looks broken.
	ollama := defaultAgentConfig("ollama")
	if ollama.Model != "qwen2.5-coder:7b" || ollama.BaseURL != "http://localhost:11434" {
		t.Fatalf("ollama config = %+v", ollama)
	}
	if ollama.MaxSteps <= 0 {
		t.Fatalf("ollama max steps = %d, want a positive loop budget", ollama.MaxSteps)
	}

	openRouter := defaultAgentConfig("9router")
	if openRouter.APIKeyEnv != "OPENROUTER_API_KEY" {
		t.Fatalf("9router API key env = %q", openRouter.APIKeyEnv)
	}

	claude := defaultAgentConfig("claude")
	if claude.APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Fatalf("claude API key env = %q", claude.APIKeyEnv)
	}
}

func TestNormalizeProvider(t *testing.T) {
	for input, want := range map[string]string{
		"ollama":     "ollama",
		"ollema":     "ollama",
		"openrouter": "9router",
		"chatgpt":    "chatgpt",
	} {
		if got := normalizeProvider(input); got != want {
			t.Errorf("normalizeProvider(%q) = %q, want %q", input, got, want)
		}
	}
}
