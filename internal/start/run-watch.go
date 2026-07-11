package start

import "fmt"

func (a StartApp) runWatch() error {
	fmt.Println("🔄 Watch mode enabled – watching for changes...")

	config, err := a.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	return runWithWatch(config)
}