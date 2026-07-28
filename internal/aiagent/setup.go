package aiagent

import (
	"fmt"
	"io"
	"strings"

	"github.com/nika-framework/nika-cli/internal/nikaconf"
)

// InitProvider configures a provider in .nika.toml.
//
// For Ollama it inspects the models that are actually installed and picks one,
// rather than writing a fixed default tag. Writing "qwen2.5-coder:7b" onto a
// machine that has never pulled it produces a config that looks fine and fails
// on the first prompt.
func InitProvider(dir, provider string, output io.Writer) error {
	name := NormalizeProvider(provider)
	if name == "" {
		return fmt.Errorf("provider is required (supported: ollama, chatgpt, 9router, claude)")
	}

	config, _, err := nikaconf.LoadFrom(dir)
	if err != nil {
		return err
	}
	agent := DefaultAgentConfig(name)

	if name == "ollama" {
		agent = resolveOllamaModel(agent, output)
	}

	config.Agent = agent
	if err := nikaconf.SaveFrom(dir, config); err != nil {
		return err
	}

	fmt.Fprintf(output, "Agent provider %q configured in %s (model: %s).\n",
		agent.Provider, nikaconf.FileName, agent.Model)
	if agent.APIKeyEnv != "" {
		fmt.Fprintf(output, "Set %s in your environment before running `nika agent`.\n", agent.APIKeyEnv)
	}
	return nil
}

// resolveOllamaModel replaces the placeholder model with an installed one.
func resolveOllamaModel(agent nikaconf.AgentConfig, output io.Writer) nikaconf.AgentConfig {
	models, err := ListOllamaModels(agent.BaseURL)
	if err != nil {
		fmt.Fprintf(output, "⚠ Could not reach Ollama (%v).\n", err)
		fmt.Fprintf(output, "  Keeping the default model %q — start Ollama and re-run this command to pick from your installed models.\n", agent.Model)
		return agent
	}
	if len(models) == 0 {
		fmt.Fprintln(output, "⚠ Ollama is running but has no models installed.")
		fmt.Fprintln(output, "  Pull one, for example: ollama pull qwen2.5-coder:7b")
		return agent
	}

	fmt.Fprintln(output, "Installed Ollama models:")
	fmt.Fprintln(output, DescribeOllamaModels(models))
	fmt.Fprintln(output)

	picked, ok := PickOllamaModel(models)
	if !ok {
		return agent
	}
	agent.Model = picked.Name

	if picked.SupportsTools() {
		fmt.Fprintf(output, "✔ Selected %s — it supports native tool calling.\n", picked.Name)
		return agent
	}
	fmt.Fprintf(output, "✔ Selected %s. It has no native tool calling, so the agent will\n", picked.Name)
	fmt.Fprintln(output, "  drive it with a JSON protocol instead. That works, but a tool-capable")
	fmt.Fprintln(output, "  model is more reliable: ollama pull qwen2.5-coder:7b")
	return agent
}

// NormalizeProvider maps the accepted spellings to canonical provider names.
func NormalizeProvider(provider string) string {
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

// DefaultAgentConfig is the starting configuration for a provider.
func DefaultAgentConfig(provider string) nikaconf.AgentConfig {
	switch provider {
	case "ollama":
		return nikaconf.AgentConfig{Provider: provider, Model: "qwen2.5-coder:7b", BaseURL: OllamaHost(), MaxSteps: 25}
	case "9router":
		return nikaconf.AgentConfig{Provider: provider, Model: "openai/gpt-4o-mini", BaseURL: "https://openrouter.ai/api/v1", APIKeyEnv: "OPENROUTER_API_KEY", MaxSteps: 25}
	case "chatgpt":
		return nikaconf.AgentConfig{Provider: provider, Model: "gpt-4o-mini", BaseURL: "https://api.openai.com/v1", APIKeyEnv: "OPENAI_API_KEY", MaxSteps: 25}
	case "claude":
		return nikaconf.AgentConfig{Provider: provider, Model: "claude-sonnet-4-5", BaseURL: "https://api.anthropic.com/v1", APIKeyEnv: "ANTHROPIC_API_KEY", MaxSteps: 25}
	default:
		return nikaconf.AgentConfig{}
	}
}

// ListModels prints the installed Ollama models and which can call tools.
func ListModels(output io.Writer, baseURL string) error {
	models, err := ListOllamaModels(baseURL)
	if err != nil {
		return err
	}
	fmt.Fprintln(output, DescribeOllamaModels(models))
	if picked, ok := PickOllamaModel(models); ok {
		fmt.Fprintf(output, "\nBest for the agent: %s\n", picked.Name)
	}
	return nil
}
