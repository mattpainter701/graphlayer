package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "graphlayer",
	Short: "GraphLayer network topology platform",
	Long: `GraphLayer is a multi-vendor network topology discovery and management platform.

It supports topology discovery from Cisco, Palo Alto, Fortinet, and other network vendors.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
