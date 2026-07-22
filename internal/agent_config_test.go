package internal

import "testing"

func TestDefaultAgentConfig(t *testing.T) {
	ollama := defaultAgentConfig("ollama")
	if ollama.Model != "gemma3:4b" || ollama.BaseURL != "http://localhost:11434" {
		t.Fatalf("ollama config = %+v", ollama)
	}

	openRouter := defaultAgentConfig("9router")
	if openRouter.APIKeyEnv != "OPENROUTER_API_KEY" {
		t.Fatalf("9router API key env = %q", openRouter.APIKeyEnv)
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
