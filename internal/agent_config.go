package internal

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

const nikaConfigPath = ".nika.toml"

type nikaConfig struct {
	Root        string      `toml:"root"`
	TestdataDir string      `toml:"testdata_dir"`
	TmpDir      string      `toml:"tmp_dir"`
	Agent       AgentConfig `toml:"agent"`
	Build       buildConfig `toml:"build"`
}

type buildConfig struct {
	Cmd          string            `toml:"cmd"`
	Args         []string          `toml:"args"`
	Bin          string            `toml:"bin"`
	Delay        int               `toml:"delay"`
	ExcludeDir   []string          `toml:"exclude_dir"`
	ExcludeFile  []string          `toml:"exclude_file"`
	ExcludeRegex []string          `toml:"exclude_regex"`
	IncludeExt   []string          `toml:"include_ext"`
	PreCmd       []string          `toml:"pre_cmd"`
	PostCmd      []string          `toml:"post_cmd"`
	Env          map[string]string `toml:"env"`
	EnvFiles     []string          `toml:"env_files"`
}

// AgentConfig is the provider configuration stored in .nika.toml.
// API keys are referenced by environment variable name and never written to disk.
type AgentConfig struct {
	Provider  string `toml:"provider"`
	Model     string `toml:"model"`
	BaseURL   string `toml:"base_url"`
	APIKeyEnv string `toml:"api_key_env"`
}

func loadNikaConfig() (nikaConfig, error) {
	var config nikaConfig
	if _, err := os.Stat(nikaConfigPath); os.IsNotExist(err) {
		return config, fmt.Errorf("%s not found; run `nika agent init <provider>` first", nikaConfigPath)
	}
	if err := func() error {
		_, err := toml.DecodeFile(nikaConfigPath, &config)
		return err
	}(); err != nil {
		return config, fmt.Errorf("read %s: %w", nikaConfigPath, err)
	}
	config.Agent.Provider = strings.ToLower(strings.TrimSpace(config.Agent.Provider))
	return config, nil
}

func initAgent(provider string) error {
	provider = normalizeProvider(provider)
	if provider == "" {
		return fmt.Errorf("provider is required (supported: ollama, 9router, chatgpt)")
	}

	config, err := loadOrCreateNikaConfig()
	if err != nil {
		return err
	}
	config.Agent = defaultAgentConfig(provider)

	file, err := os.Create(nikaConfigPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", nikaConfigPath, err)
	}
	defer file.Close()
	if _, err := file.WriteString("#:schema https://json.schemastore.org/any.json\n\n"); err != nil {
		return err
	}
	if err := toml.NewEncoder(file).Encode(config); err != nil {
		return fmt.Errorf("write %s: %w", nikaConfigPath, err)
	}
	return nil
}

func loadOrCreateNikaConfig() (nikaConfig, error) {
	if _, err := os.Stat(nikaConfigPath); err == nil {
		return loadNikaConfig()
	} else if !os.IsNotExist(err) {
		return nikaConfig{}, err
	}
	return nikaConfig{
		Root:        ".",
		TestdataDir: "testdata",
		TmpDir:      "tmp",
		Build: buildConfig{
			Cmd:          "go run .",
			Delay:        1000,
			ExcludeDir:   []string{"docs", "tmp", "vendor", "testdata", ".git", "cache"},
			ExcludeRegex: []string{"^\\."},
			IncludeExt:   []string{".go"},
			Env:          map[string]string{},
		},
	}, nil
}

func defaultAgentConfig(provider string) AgentConfig {
	switch provider {
	case "ollama":
		return AgentConfig{Provider: provider, Model: "gemma3:4b", BaseURL: "http://localhost:11434"}
	case "9router":
		return AgentConfig{Provider: provider, Model: "openai/gpt-4o-mini", BaseURL: "https://openrouter.ai/api/v1", APIKeyEnv: "OPENROUTER_API_KEY"}
	case "chatgpt":
		return AgentConfig{Provider: provider, Model: "gpt-4o-mini", BaseURL: "https://api.openai.com/v1", APIKeyEnv: "OPENAI_API_KEY"}
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
	default:
		return ""
	}
}
