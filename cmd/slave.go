package cmd

import (
	bootstrap "github.com/golang-projects/master_slave/pkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
	"strings"
)

const (
	MasterHost = "master-host"
	PprofHost  = "pprof-host"
)

// slaveCmd represents the slave command
var (
	slaveOpts *bootstrap.SlaveOpts
	slaveCmd  = &cobra.Command{
		Use:   "slave",
		Short: "Start slave http servers",
		Long: `Slave will startup and send a registration request to the master. Slave will also start an http
server which exposes pprof endpoints for the master to fetch that data from slave`,
		Run: func(cmd *cobra.Command, args []string) {
			s, err := bootstrap.NewSlave(slaveOpts)
			done := make(chan struct{})
			if err != nil {
				log.Fatalf("failed to get new slave: %v", err)
			}
			if err := s.Start(done); err != nil {
				close(done)
				log.Fatalf("failed to bootstrap slave: %v", err)
			}
		},
	}
)

func init() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	slaveCmd.Flags().String(MasterHost, "localhost:9080", "master grpc server host:port")
	if err := viper.BindPFlag(MasterHost, slaveCmd.Flags().Lookup(MasterHost)); err != nil {
		log.Fatalf("viper bind flag error: %v", err)
	}
	slaveCmd.Flags().String(PprofHost, "localhost:8080", "master grpc server host:port")
	if err := viper.BindPFlag(PprofHost, slaveCmd.Flags().Lookup(PprofHost)); err != nil {
		log.Fatalf("viper bind flag error: %v", err)
	}
	slaveOpts = &bootstrap.SlaveOpts{}
	slaveOpts.MasterHost = viper.GetString(MasterHost)
	slaveOpts.PprofServerHost = viper.GetString(PprofHost)
	rootCmd.AddCommand(slaveCmd)
}
