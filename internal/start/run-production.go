package start

import (
	"fmt"
	"os"
	"os/exec"
)

// runGo executes "go run" once for the given target (file or directory)
func (a StartApp) runProduction() error {
	cmd := exec.Command("go", "run", a.Target)
	cmd.Dir = a.Target // if target is a dir, set it as working directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("go run failed: %w", err)
	}
	return nil
}