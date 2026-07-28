package internal

import (
	"fmt"
	"io"
)

// InitAgent configures the selected AI provider in .nika.toml.
//
// Running a prompt lives in internal/aiagent now: the keyword dispatch that
// used to sit here could only reach two fixed generators, so anything else the
// user asked for was answered with prose and no edits.
func InitAgent(provider string, output io.Writer) error {
	if err := initAgent(provider); err != nil {
		return err
	}
	config, err := loadNikaConfig()
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "Agent provider %q configured in %s (model: %s).\n",
		config.Agent.Provider, nikaConfigPath, config.Agent.Model); err != nil {
		return err
	}
	if config.Agent.APIKeyEnv != "" {
		_, err = fmt.Fprintf(output, "Set %s in your environment before running `nika agent`.\n", config.Agent.APIKeyEnv)
	}
	return err
}
