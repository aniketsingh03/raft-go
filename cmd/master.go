package main

import (
	bootstrap "github.com/golang-projects/master_slave/pkg"
	"github.com/spf13/cobra"
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
		// TODO convert this to a CRD and manage from k8s yaml
		opts := &bootstrap.MasterOpts{
			GRPCAddr: "127.0.0.1:8080",
		}
		s.Start(opts)
	},
}
