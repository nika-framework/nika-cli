package start

import "fmt"

func (a StartApp) Run() error {
	resolved, err := a.resolve()
	if err != nil {
		return err
	}
	if resolved.App != "" {
		fmt.Printf("▶️  Starting %s: %s\n", resolved.App, resolved.Build.Cmd)
	}
	if a.WatchMode {
		return a.runWatch(resolved)
	}
	return a.runProduction(resolved)
}
