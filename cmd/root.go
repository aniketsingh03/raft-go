package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"os"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "A leader follower architectural pattern",
	Long: `Start a leader-follower pattern where leader election, log replication and compaction happens on the basis
			of raft consensus`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		log.Print("starting servers...")
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		log.Printf("Failed to execute cobra command, error: %v", err)
		os.Exit(1)
	}
}
