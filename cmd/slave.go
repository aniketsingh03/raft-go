package cmd

import (
	"fmt"
	"github.com/ghodss/yaml"
	bootstrap "github.com/golang-projects/master_slave/pkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
	"log"
	"strings"
	"sync"
)

const (
	NumberOfSlaves = "num"
)

// slaveCmd represents the slave command
var (
	slaveCmd = &cobra.Command{
		Use:   "slaves",
		Short: "Start 1 or more slaves",
		Long: `This command will start [num] number of slaves and expose pprof endpoints for stats, 
and once they have started up, will send registration requests to the master for registering each one of them
master host:port, pprof host:port, slave configs (name, host:port) will be picked up from respective ymls, eg:
slave-0.yml and slave-1.yml`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("number: %v", viper.GetInt(NumberOfSlaves))
			var wg sync.WaitGroup
			for i := 0; i < viper.GetInt(NumberOfSlaves); i++ {
				wg.Add(1)
				slaveOpts := &bootstrap.SlaveOpts{}
				slaveConfigYAMLPath := fmt.Sprintf("cmd/configs/slave-%v.yaml", i)
				yamlFileBytes, err := ioutil.ReadFile(slaveConfigYAMLPath)
				if err != nil {
					log.Fatalf("Failed to read file: %v, err: %v", slaveConfigYAMLPath, err)
				}
				if err := yaml.Unmarshal(yamlFileBytes, slaveOpts); err != nil {
					log.Fatalf("Failed to unmarshal %v, err: %v", slaveConfigYAMLPath, err)
				}
				s, err := bootstrap.NewSlave(slaveOpts)
				done := make(chan struct{})
				if err != nil {
					log.Fatalf("failed to get new slave: %v", err)
				}
				go func(opts *bootstrap.SlaveOpts) {
					defer wg.Done()
					if err := s.Start(done, opts); err != nil {
						close(done)
						log.Fatalf("failed to bootstrap slave: %v", err)
					}
				}(slaveOpts)
			}
			wg.Wait()
		},
	}
)

func init() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	slaveCmd.Flags().String(NumberOfSlaves, "1", "number of slaves to start")
	if err := viper.BindPFlag(NumberOfSlaves, slaveCmd.Flags().Lookup(NumberOfSlaves)); err != nil {
		log.Fatalf("viper bind flag error: %v", err)
	}
	rootCmd.AddCommand(slaveCmd)
}
