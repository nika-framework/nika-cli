package start

import (
	"github.com/nika-framework/nika-cli/internal/nikaconf"
)

// Config and BuildConfig are the shared .nika.toml types. They are aliased
// rather than redeclared so `nika start` writing the file cannot drop the
// [workspace] or [agent] sections it does not itself read.
type Config = nikaconf.Config

// BuildConfig holds build and watch related settings.
type BuildConfig = nikaconf.BuildConfig

// defaultConfig returns the default configuration values.
func defaultConfig() Config { return nikaconf.Default() }

// LoadConfig reads .nika.toml, creating it with defaults when absent.
func (a StartApp) LoadConfig() (Config, error) {
	config, exists, err := nikaconf.Load(a.PathTom)
	if err != nil {
		return config, err
	}
	if !exists {
		config = defaultConfig()
		if err := a.saveConfig(config); err != nil {
			return config, err
		}
	}
	return config, nil
}

// saveConfig writes the config struct to the toml file.
func (a StartApp) saveConfig(config Config) error {
	return nikaconf.Save(a.PathTom, config)
}
