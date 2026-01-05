package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "poxy",
	Short: "Poxy is a universal package manager for linux distrobutions",
	Long:  `Poxy discover's linux distro's packaging system and unifies each command based on the systems package manager`,
}

func init() {
	rootCmd.AddCommand(versionCmd)

}
func Execute() error {
	return rootCmd.Execute()
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints current version number for Poxy",
	Run: func(cmd *cobra.Command, args []string) {
		version := "0.1.0"
		versionStr := fmt.Sprintf("Version:  %s", version)
		fmt.Println(versionStr)
	},
}
