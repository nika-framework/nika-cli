//go:build windows

package start

import "os/exec"

func configureProcess(cmd *exec.Cmd) {
	// Windows does not support Unix process groups or syscall.Kill.
}

func stopProcess(cmd *exec.Cmd) {
	_ = cmd.Process.Kill()
}
