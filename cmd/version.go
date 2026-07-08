package cmd

import (

	"github.com/nika-framework/nika-cli/internal"
	"github.com/spf13/cobra"
)



var versionCmd = &cobra.Command{
	Use:   "version", 
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		internal.RunCheckVersion(cmd,args)
	},
}
var vCmd = &cobra.Command{
	Use:   "v", 
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		internal.RunCheckVersion(cmd,args)
	},
}

func init() {
	 rootCmd.AddCommand(versionCmd)
	 rootCmd.AddCommand(vCmd)
}