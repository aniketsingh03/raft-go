package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"os"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Start a master-slave architecture",
	Long:  `Start a master-slave architecture with a registration server at master's end`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		log.Print("starting master and slave servers...")
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		log.Printf("Failed to execute cobra command, error: %v", err)
		os.Exit(1)
	}
}
