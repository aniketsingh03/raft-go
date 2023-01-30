package cmd

import (
	bootstrap "github.com/golang-projects/master_slave/pkg"
	"github.com/spf13/cobra"
	"log"
)

// masterCmd represents the server command
var masterCmd = &cobra.Command{
	Use:   "server",
	Short: "Start master gRPC server",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		s := bootstrap.NewMasterServer()
		done := make(chan struct{})
		// start the master gRPC server at localhost:9080
		opts := &bootstrap.MasterOpts{
			GRPCAddr: "localhost:9080",
		}
		if err := s.Start(opts, done); err != nil {
			close(done)
			log.Fatalf("Failed to start master server, err: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(masterCmd)
}
