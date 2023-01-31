package cmd

import (
	"fmt"
	"github.com/ghodss/yaml"
	bootstrap "github.com/golang-projects/master_slave/pkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	NumberOfServers = "num"
)

// serverCmd represents the slave command
var (
	serverCmd = &cobra.Command{
		Use:   "servers",
		Short: "Start 1 or more servers",
		Long:  `This command will start [num] number of servers and expose pprof endpoints for stats`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("number: %v", viper.GetInt(NumberOfServers))
			slaveServers := make([]*bootstrap.RaftServer, 0)
			var d chan struct{}
			for i := 1; i <= viper.GetInt(NumberOfServers); i++ {
				serverOpts := &bootstrap.ServerOpts{}
				serverConfigYAMLPath := fmt.Sprintf("cmd/configs/server-%v.yaml", i)
				yamlFileBytes, err := ioutil.ReadFile(serverConfigYAMLPath)
				if err != nil {
					log.Fatalf("Failed to read file: %v, err: %v", serverConfigYAMLPath, err)
				}
				if err := yaml.Unmarshal(yamlFileBytes, serverOpts); err != nil {
					log.Fatalf("Failed to unmarshal %v, err: %v", serverConfigYAMLPath, err)
				}
				s, err := bootstrap.NewRaftServer(serverOpts)
				slaveServers = append(slaveServers, s)
				done := make(chan struct{})
				d = done
				if err != nil {
					log.Fatalf("failed to get new slave: %v", err)
				}
				go func(opts *bootstrap.ServerOpts) {
					if err := s.Start(done, opts); err != nil {
						close(done)
						log.Fatalf("failed to bootstrap slave: %v", err)
					}
				}(serverOpts)
			}
			// wait for all grpc and http servers in all servers to come up, and then call RequestVote RPC
			time.Sleep(2 * time.Second)

			log.Print("started all http and grpc servers.....")
			log.Printf("size: %v", len(slaveServers))
			for _, s := range slaveServers {
				log.Printf("AT COBRA, SLAVE SERVER %v", s.Name)
				go func(s *bootstrap.RaftServer) {
					s.SetTimeoutAndStartVoteRequest()
				}(s)
			}
			waitSignal(d)
		},
	}
)

func waitSignal(stop chan struct{}) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	close(stop)
}

func init() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	serverCmd.Flags().String(NumberOfServers, "5", "number of slaves to start")
	if err := viper.BindPFlag(NumberOfServers, serverCmd.Flags().Lookup(NumberOfServers)); err != nil {
		log.Fatalf("viper bind flag error: %v", err)
	}
	rootCmd.AddCommand(serverCmd)
}
