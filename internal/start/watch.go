package start

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// runWithWatch runs the project and restarts it on file changes using fsnotify
func runWithWatch(config Config) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Compile exclude regex patterns once
	excludeRegexes := make([]*regexp.Regexp, 0, len(config.Build.ExcludeRegex))
	for _, pattern := range config.Build.ExcludeRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid exclude_regex %q: %w", pattern, err)
		}
		excludeRegexes = append(excludeRegexes, re)
	}

	err = addDirRecursively(watcher, config.Root, config.Build.ExcludeDir)
	if err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	var currentCmd *exec.Cmd
	var debounceTimer *time.Timer
	var currentDone chan struct{}
	// Delay comes from config (milliseconds). Fallback to 1000ms if not set.
	delay := time.Duration(config.Build.Delay) * time.Millisecond
	if delay <= 0 {
		delay = 1 * time.Second
	}

	// Function to completely terminate the processes
	stopCurrentCmd := func() {
		if currentCmd != nil && currentCmd.Process != nil {
			stopProcess(currentCmd)
			if currentDone != nil {
				<-currentDone
			}
		}
		currentCmd = nil
		currentDone = nil
	}

	runCmdList := func(cmds []string) {
		for _, c := range cmds {
			if strings.TrimSpace(c) == "" {
				continue
			}
			fmt.Printf("⚙️  Running: %s\n", c)
			parts := strings.Fields(c)
			cmd := exec.Command(parts[0], parts[1:]...)
			cmd.Dir = config.Root
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Env = buildEnv(config)
			if err := cmd.Run(); err != nil {
				fmt.Printf("⚠️  Command failed: %s (%v)\n", c, err)
			}
		}
	}

	startApp := func() {
		stopCurrentCmd()

		// Run pre_cmd hooks
		runCmdList(config.Build.PreCmd)

		fmt.Println("🚀 Starting application...")

		cmdName := config.Build.Cmd
		if cmdName == "" {
			cmdName = "go run"
		}
		cmdParts := strings.Fields(cmdName)
		args := append(cmdParts[1:], config.Build.Args...)

		cmd := exec.Command(cmdParts[0], args...)
		cmd.Dir = config.Root
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = buildEnv(config)

		configureProcess(cmd)

		if err := cmd.Start(); err != nil {
			fmt.Printf("❌ Failed to start application: %v\n", err)
			return
		}
		currentCmd = cmd
		done := make(chan struct{})
		currentDone = done

		go func() {
			err := cmd.Wait()
			if err != nil {
				if _, ok := err.(*exec.ExitError); !ok {
					fmt.Printf("⚠️ Application exited with error: %v\n", err)
				}
			}
			if currentCmd == cmd {
				currentCmd = nil
			}
			// Run post_cmd hooks after process exits
			runCmdList(config.Build.PostCmd)
			close(done)
		}()
	}

	startApp()

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				if !shouldWatch(event.Name, config, excludeRegexes) {
					continue
				}
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(delay, func() {
					fmt.Printf("📁 Change detected: %s\n", event.Name)
					startApp()
				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Printf("⚠️ Watcher error: %v\n", err)

		case <-sigChan:
			fmt.Println("\n🛑 Shutting down...")
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			stopCurrentCmd()
			return nil
		}
	}
}

// shouldWatch decides whether a changed file should trigger a restart,
// based on include_ext, exclude_file and exclude_regex from the config.
func shouldWatch(path string, config Config, excludeRegexes []*regexp.Regexp) bool {
	base := filepath.Base(path)

	// exclude_file: exact file name match
	for _, ef := range config.Build.ExcludeFile {
		if base == ef {
			return false
		}
	}

	// exclude_regex: match against file name
	for _, re := range excludeRegexes {
		if re.MatchString(base) {
			return false
		}
	}

	// include_ext: only watch files with these extensions (if configured)
	if len(config.Build.IncludeExt) > 0 {
		ext := filepath.Ext(base)
		matched := false
		for _, ie := range config.Build.IncludeExt {
			if ext == ie {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// buildEnv merges current process env with config.Build.Env
func buildEnv(config Config) []string {
	env := os.Environ()
	for k, v := range config.Build.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

// addDirRecursively adds a directory and all its subdirectories to the watcher,
// skipping any directories listed in excludeDirs.
func addDirRecursively(watcher *fsnotify.Watcher, dir string, excludeDirs []string) error {
	return walkDir(dir, excludeDirs, func(path string) error {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := watcher.Add(path); err != nil {
				return err
			}
		}
		return nil
	})
}

// isExcludedDir checks if a directory name is in the exclude list
func isExcludedDir(name string, excludeDirs []string) bool {
	for _, ex := range excludeDirs {
		if name == ex {
			return true
		}
	}
	return false
}

// walkDir is a simple recursive walker that skips excluded directories
func walkDir(root string, excludeDirs []string, fn func(path string) error) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() && isExcludedDir(entry.Name(), excludeDirs) {
			continue
		}
		fullPath := filepath.Join(root, entry.Name())
		if entry.IsDir() {
			if err := walkDir(fullPath, excludeDirs, fn); err != nil {
				return err
			}
		}
		if err := fn(fullPath); err != nil {
			return err
		}
	}
	return fn(root)
}
