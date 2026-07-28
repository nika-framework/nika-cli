package start

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// runProduction runs the resolved build command once, in the foreground.
//
// The previous version ran `go run <target>` with Dir also set to <target>,
// which for the default "./main.go" meant chdir'ing into a file — so a plain
// `nika start` failed before it ever reached the compiler.
func (a StartApp) runProduction(resolved plan) error {
	command := strings.TrimSpace(resolved.Build.Cmd)
	if command == "" {
		command = "go run ."
	}
	parts := strings.Fields(command)
	args := append(append([]string{}, parts[1:]...), resolved.Build.Args...)

	cmd := exec.Command(parts[0], args...)
	cmd.Dir = resolved.Config.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = buildEnv(resolved.Build)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", command, err)
	}
	return nil
}
