package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

var watchMode bool // flag for watch mode

// startCmd defines the start command
var startCmd = &cobra.Command{
	Use:   "start [file-or-dir]",
	Short: "Start Nika application",
	Long:  `Runs the Go project in the given directory or file.`,
	Args:  cobra.ExactArgs(1), // exactly one argument (path to the project or main.go)
	RunE: func(cmd *cobra.Command, args []string) error {
		target := args[0]

		if watchMode {
			fmt.Println("🔄 Watch mode enabled – watching for changes...")
			return runWithWatch(target)
		}

		// Normal mode: just run once
		return runGo(target)
	},
}

// runGo executes "go run" once for the given target (file or directory)
func runGo(target string) error {
	cmd := exec.Command("go", "run", target)
	cmd.Dir = target // if target is a dir, set it as working directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("go run failed: %w", err)
	}
	return nil
}

// runWithWatch runs the project and restarts it on file changes using fsnotify
func runWithWatch(target string) error {
	// Create a new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	// Channel to signal when to stop the running process
	stopChan := make(chan struct{})
	// Channel to receive OS signals (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start watching the target directory recursively
	// For simplicity, we watch the target directory and all subdirectories.
	// You may want to limit to .go files only, but watching everything is fine.
	err = addDirRecursively(watcher, target)
	if err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	// Variable to hold the current running command
	var currentCmd *exec.Cmd

	// Function to stop the currently running command
	stopCurrentCmd := func() {
		if currentCmd != nil && currentCmd.Process != nil {
			// Kill the process group to also kill child processes
			if err := currentCmd.Process.Kill(); err != nil {
				fmt.Printf("⚠️ Failed to kill process: %v\n", err)
			}
			currentCmd = nil
		}
	}

	// Function to start (or restart) the application
	startApp := func() {
		stopCurrentCmd() // stop any previous instance

		fmt.Println("🚀 Starting application...")
		cmd := exec.Command("go", "run", target)
		cmd.Dir = target
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			fmt.Printf("❌ Failed to start application: %v\n", err)
			return
		}
		currentCmd = cmd

		// Wait for the command to finish in a goroutine (so we can handle restarts)
		go func() {
			err := cmd.Wait()
			if err != nil {
				// If the process was killed by us, ignore the error
				if _, ok := err.(*exec.ExitError); ok {
					// It's a normal exit due to kill or termination
				} else {
					fmt.Printf("⚠️ Application exited with error: %v\n", err)
				}
			} else {
				fmt.Println("✅ Application stopped gracefully")
			}
			// If the command finished without being killed by us, clear currentCmd
			if currentCmd == cmd {
				currentCmd = nil
			}
		}()
	}

	// Start the app for the first time
	startApp()

	// Main loop: listen for events and signals
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// On any write or create event, restart the app
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				// Avoid restarting on temporary files or hidden files (optional)
				fmt.Printf("📁 Change detected: %s, restarting...\n", event.Name)
				startApp()
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Printf("⚠️ Watcher error: %v\n", err)

		case <-sigChan:
			// Received SIGINT or SIGTERM: stop the app and exit
			fmt.Println("\n🛑 Shutting down...")
			stopCurrentCmd()
			return nil

		case <-stopChan:
			// Internal stop signal (not used here, but available)
			stopCurrentCmd()
			return nil
		}
	}
}

// addDirRecursively adds a directory and all its subdirectories to the watcher
func addDirRecursively(watcher *fsnotify.Watcher, dir string) error {
	// Walk the directory tree
	return walkDir(dir, func(path string) error {
		// Only add directories (not files)
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

// walkDir is a simple recursive walker (similar to filepath.Walk)
func walkDir(root string, fn func(path string) error) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		fullPath := root + string(os.PathSeparator) + entry.Name()
		if entry.IsDir() {
			// Recursively walk subdirectories
			if err := walkDir(fullPath, fn); err != nil {
				return err
			}
		}
		// Call the function for the current entry
		if err := fn(fullPath); err != nil {
			return err
		}
	}
	// Also call fn for the root itself
	return fn(root)
}

func init() {
	// Define the --watch flag (boolean)
	startCmd.Flags().BoolVar(&watchMode, "watch", false, "Run in watch mode (auto-restart on changes)")
	rootCmd.AddCommand(startCmd)
}