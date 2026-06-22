package cmd

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newCmd defines the "new" Cobra command that creates a new Nika application.
var newCmd = &cobra.Command{
	Use:   "new [app-name]",
	Short: "Create a new Nika application",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) <= 0 {
			fmt.Println("❌ Error: requires exactly one argument - the application name.")
			return
		}
		appName := args[0] 
		appName = strings.TrimSpace(appName) 
		appName = strings.ReplaceAll(appName, " ", "-") 
		appName = strings.ToLower(appName)
		re := regexp.MustCompile(`[^a-z0-9\-_]`)
		appName = re.ReplaceAllString(appName, "")
		if appName == "" {
			fmt.Println("❌ Error: Invalid application name")
			return
		} 
		done := make(chan error, 1)
		gitCmd := exec.Command("git", "clone", "https://github.com/sajadweb/go-module.git", "./"+appName)
		if err := gitCmd.Start(); err != nil {
			fmt.Println("Failed to start git:", err)
			return
		}
		go func() {
			done <- gitCmd.Wait()
		}()
		spinner := []rune{'|', '/', '-', '\\'}
		i := 0
		fmt.Print("Cloning repository ")
	loop:
		for {
			select {
			case err := <-done:
				if err != nil {
					fmt.Printf("\rClone failed: %v\n", err)
					return
				}
				fmt.Printf("\rClone completed.          \n")
				break loop
			default:
				fmt.Printf("\rCloning repository %c", spinner[i%len(spinner)])
				i++
				time.Sleep(120 * time.Millisecond)
			}
		}

		fmt.Println("📦 Project structure created successfully!")
		fmt.Println("✅ Done! Your app is ready.")
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}
