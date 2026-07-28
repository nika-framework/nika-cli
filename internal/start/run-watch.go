package start

import "fmt"

func (a StartApp) runWatch(resolved plan) error {
	fmt.Println("🔄 Watch mode enabled – watching for changes...")
	return runWithWatch(resolved)
}
