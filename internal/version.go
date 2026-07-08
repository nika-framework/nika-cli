package internal

import (
	"fmt"
	"runtime/debug"

	"github.com/sajadweb/nika-cli/common"
	"github.com/spf13/cobra"
)

// Set via ldflags
var (
	version = "0.1.1" 
)

// RunNewProject executes the full step-by-step project creation flow.
func RunCheckVersion(cmd *cobra.Command, args []string) error {
	sp := common.NewSpinner()
	// ── Step 1: Validate name ──────────────────────────────────────
	common.Section("Nika Version")
	sp.Start("...")
	fmt.Fprintf(cmd.OutOrStdout(), "Nika %s  \n", version,)
	if info, ok := debug.ReadBuildInfo(); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "go: %s\n", info.GoVersion)
	}
	sp.Stop("Done") 
	return nil
}
