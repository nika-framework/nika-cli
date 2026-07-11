package start

import (
	"os"

	"github.com/BurntSushi/toml"
)

// Config represents the .nika.toml configuration structure
type Config struct {
	Root        string      `toml:"root"`
	TestdataDir string      `toml:"testdata_dir"`
	TmpDir      string      `toml:"tmp_dir"`
	Build       BuildConfig `toml:"build"`
}

// BuildConfig holds build and watch related settings
type BuildConfig struct {
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

// defaultConfig returns the default configuration values
func defaultConfig() Config {
	return Config{
		Root:        ".",
		TestdataDir: "testdata",
		TmpDir:      "tmp",
		Build: BuildConfig{
			Cmd:          "go run .",
			Args:         []string{},
			Bin:          "",
			Delay:        1000,
			ExcludeDir:   []string{"docs", "tmp", "vendor", "testdata", ".git", "cache"},
			ExcludeFile:  []string{},
			ExcludeRegex: []string{`^\.`},
			IncludeExt:   []string{".go"},
			PreCmd:       []string{},
			PostCmd:      []string{},
			Env:          map[string]string{},
			EnvFiles:     []string{},
		},
	}
}

func (a StartApp) LoadConfig() (Config, error) {
	var config Config

	// If the config file doesn't exist, create it with default values
	if _, err := os.Stat(a.PathTom); os.IsNotExist(err) {
		config = defaultConfig()
		if err := a.saveConfig(config); err != nil {
			return config, err
		}
		return config, nil
	}

	// File exists, load it
	if _, err := toml.DecodeFile(a.PathTom, &config); err != nil {
		return config, err
	}
	return config, nil
}

// saveConfig writes the config struct to the toml file
func (a StartApp) saveConfig(config Config) error {
	f, err := os.Create(a.PathTom)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write schema comment header
	if _, err := f.WriteString("#:schema https://json.schemastore.org/any.json\n\n"); err != nil {
		return err
	}

	encoder := toml.NewEncoder(f)
	return encoder.Encode(config)
}
