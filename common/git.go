package common

import (
	"fmt"
	"os/exec"
	"runtime"
)

// IsGitAvailable checks whether the `git` binary is on PATH.
func IsGitAvailable() bool {
	name := "git"
	if runtime.GOOS == "windows" {
		name = "git.exe"
	}
	_, err := exec.LookPath(name)
	return err == nil
}

// GitInit runs `git init` inside the given directory.
func GitInit(dir string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init failed: %w\n%s", err, out)
	}
	return nil
}

// GitClone clones a repository into the target directory.
func GitClone(repoURL, targetDir string) error {
	cmd := exec.Command("git", "clone", repoURL, targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w\n%s", err, out)
	}
	return nil
}

// RemoveGitDir deletes the .git directory inside the given path.
func RemoveGitDir(dir string) error {
	cmd := exec.Command("rm", "-rf", dir+"/.git")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("removing .git failed: %w\n%s", err, out)
	}
	return nil
}
