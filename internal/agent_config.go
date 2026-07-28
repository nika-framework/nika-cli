package internal

import (
	"fmt"
	"os"
	"strings"

	"github.com/nika-framework/nika-cli/internal/nikaconf"
)

const nikaConfigPath = nikaconf.FileName

// AgentConfig re-exports the provider configuration so existing callers keep
// compiling after the config format moved into its own package.
type AgentConfig = nikaconf.AgentConfig

func loadNikaConfig() (nikaconf.Config, error) {
	config, exists, err := nikaconf.Load(nikaConfigPath)
	if err != nil {
		return config, err
	}
	if !exists {
		return config, fmt.Errorf("%s not found; run `nika agent init <provider>` first", nikaConfigPath)
	}
	return config, nil
}

func initAgent(provider string) error {
	provider = normalizeProvider(provider)
	if provider == "" {
		return fmt.Errorf("provider is required (supported: ollama, 9router, chatgpt, claude)")
	}

	config, err := loadOrCreateNikaConfig()
	if err != nil {
		return err
	}
	config.Agent = defaultAgentConfig(provider)
	return nikaconf.Save(nikaConfigPath, config)
}

func loadOrCreateNikaConfig() (nikaconf.Config, error) {
	if _, err := os.Stat(nikaConfigPath); err == nil {
		return loadNikaConfig()
	} else if !os.IsNotExist(err) {
		return nikaconf.Config{}, err
	}
	return nikaconf.Default(), nil
}

func defaultAgentConfig(provider string) AgentConfig {
	switch provider {
	case "ollama":
		// A tool-calling capable default: the agent loop is useless with a
		// model that cannot emit function calls.
		return AgentConfig{Provider: provider, Model: "qwen2.5-coder:7b", BaseURL: "http://localhost:11434", MaxSteps: 25}
	case "9router":
		return AgentConfig{Provider: provider, Model: "openai/gpt-4o-mini", BaseURL: "https://openrouter.ai/api/v1", APIKeyEnv: "OPENROUTER_API_KEY", MaxSteps: 25}
	case "chatgpt":
		return AgentConfig{Provider: provider, Model: "gpt-4o-mini", BaseURL: "https://api.openai.com/v1", APIKeyEnv: "OPENAI_API_KEY", MaxSteps: 25}
	case "claude":
		return AgentConfig{Provider: provider, Model: "claude-sonnet-4-5", BaseURL: "https://api.anthropic.com/v1", APIKeyEnv: "ANTHROPIC_API_KEY", MaxSteps: 25}
	default:
		return AgentConfig{}
	}
}

func normalizeProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "ollama", "ollema":
		return "ollama"
	case "9router", "openrouter":
		return "9router"
	case "chatgpt", "openai":
		return "chatgpt"
	case "claude", "anthropic":
		return "claude"
	default:
		return ""
	}
}
