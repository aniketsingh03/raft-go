package cmd

import (
	"fmt"
	"github.com/ghodss/yaml"
	bootstrap "github.com/golang-projects/master_slave/pkg"
	"github.com/spf13/cobra"
	"io/ioutil"
	"log"
)

const (
	masterConfigYamlPath = "master.yaml"
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
		// start the master gRPC server at the host specified in master.yaml
		masterOpts := &bootstrap.MasterOpts{}
		yamlFileBytes, err := ioutil.ReadFile(fmt.Sprintf("cmd/configs/%v", masterConfigYamlPath))
		if err != nil {
			log.Fatalf("Failed to read file: %v, err: %v", masterConfigYamlPath, err)
		}
		if err := yaml.Unmarshal(yamlFileBytes, masterOpts); err != nil {
			log.Fatalf("Failed to unmarshal %v, err: %v", masterConfigYamlPath, err)
		}
		if err := s.Start(masterOpts, done); err != nil {
			close(done)
			log.Fatalf("Failed to start master server, err: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(masterCmd)
}
