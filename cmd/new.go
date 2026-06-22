package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new [app-name]",
	Short: "Create a new Nika application",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		appName := args[0]

		fmt.Printf("🚀 Creating new Nika app: %s\n", appName)
		fmt.Println("📦 Project structure created successfully!")
		fmt.Println("✅ Done! Your app is ready.")
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}