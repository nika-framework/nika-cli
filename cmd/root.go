package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "nika",
	Short: "Nika CLI - backend framework tool",
	Long:  "CLI tool for Nika framework (Go backend framework)",
	// A failing command is almost never a usage mistake, and dumping the full
	// help text after a real error buries it. Errors are reported once, by
	// Execute, on stderr.
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
