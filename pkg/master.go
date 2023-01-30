package pkg

import (
	"context"
	"fmt"
	pb "github.com/golang-projects/master_slave/pkg/api"
	"google.golang.org/grpc"
	"log"
	"net"
)

type SlaveInfo struct {
	slaveHTTPServerHost string
	slaveName           string
}

type MasterServer struct {
	pb.DiscoveryServer
	grpcServer *grpc.Server
	slaveInfo  map[string]*SlaveInfo
}

// hostname:port combination where master should listen
type MasterOpts struct {
	// listening address for gRPC servers of both master and slave
	GRPCAddr string `json:"grpc_addr"`
}

func NewMasterServer() *MasterServer {
	return &MasterServer{
		slaveInfo: map[string]*SlaveInfo{},
	}
}

func (s *MasterServer) Start(opts *MasterOpts, stop chan struct{}) error {
	listener, err := net.Listen("tcp", opts.GRPCAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s.grpcServer = grpc.NewServer()
	pb.RegisterDiscoveryServer(s.grpcServer, s)
	log.Printf("GRPC server listening on %v", listener.Addr())
	if err := s.grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}

	s.waitForShutdown(stop)
	return nil
}

func (s *MasterServer) waitForShutdown(stop chan struct{}) {
	go func() {
		<-stop
		if s.grpcServer != nil {
			s.grpcServer.Stop()
		}
	}()
}

// RegisterSlave is called when a slave starts up and sends pings to master for registering itself
func (s *MasterServer) RegisterSlave(ctx context.Context, req *pb.RegisterSlaveRequest) (*pb.RegisterSlaveResponse, error) {
	log.Printf("received message %v from slave %v", req.Message, req.SlaveName)
	s.slaveInfo[req.SlaveName] = &SlaveInfo{
		slaveHTTPServerHost: req.SlaveHttpServerHost,
		slaveName:           req.SlaveName,
	}
	return &pb.RegisterSlaveResponse{Message: fmt.Sprintf("Registered slave %v!", req.SlaveName)}, nil
}
