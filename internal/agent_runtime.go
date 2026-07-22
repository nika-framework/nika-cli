package internal

import (
	"fmt"
	"io"
	"strings"
)

// InitAgent configures the selected AI provider in .nika.toml.
func InitAgent(provider string, output io.Writer) error {
	if err := initAgent(provider); err != nil {
		return err
	}
	config, err := loadNikaConfig()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Agent provider %q configured in %s (model: %s).\n", config.Agent.Provider, nikaConfigPath, config.Agent.Model)
	return err
}

// RunConfiguredAgent executes a prompt using the provider configured in .nika.toml.
func RunConfiguredAgent(prompt string, output io.Writer) error {
	config, err := loadNikaConfig()
	if err != nil {
		return err
	}
	runtime := agentRuntime{
		Provider:  normalizeProvider(config.Agent.Provider),
		Model:     strings.TrimSpace(config.Agent.Model),
		BaseURL:   strings.TrimSpace(config.Agent.BaseURL),
		APIKeyEnv: strings.TrimSpace(config.Agent.APIKeyEnv),
	}
	if runtime.Provider == "" {
		return fmt.Errorf("unknown agent provider %q", config.Agent.Provider)
	}
	if runtime.Model == "" {
		return fmt.Errorf("agent model is empty in %s", nikaConfigPath)
	}
	if isRoutePromptInternal(prompt) {
		return runOllemaRoute(runtime, prompt, output)
	}
	if isModulePromptInternal(prompt) {
		return runOllemaModule(runtime, prompt, output)
	}
	response, err := askAgent(runtime, prompt, "")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, response.Response)
	return err
}

func isRoutePromptInternal(prompt string) bool {
	prompt = strings.ToLower(prompt)
	for _, marker := range []string{"route", "روت", "mock", "ماک", "دیتای ماک", "endpoint"} {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

func isModulePromptInternal(prompt string) bool {
	prompt = strings.ToLower(prompt)
	for _, marker := range []string{"module", "ماژول", "بساز", "ساخت", "build", "create", "generate"} {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}
