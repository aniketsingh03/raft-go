package pkg

import (
	"context"
	pb "github.com/golang-projects/master_slave/pkg/api"
	"google.golang.org/grpc"
	"log"
	"net"
)

type SlaveInfo struct {
	port      uint32
	slaveUUID string
}

type MasterServer struct {
	pb.MasterServer
	slaveInfo map[string]*SlaveInfo
}

// hostname:port combination where master should listen
type MasterOpts struct {
	// listening address for gRPC servers of both master and slave
	GRPCAddr string
}

func NewMasterServer() *MasterServer {
	return &MasterServer{
		slaveInfo: map[string]*SlaveInfo{},
	}
}

func (s *MasterServer) Start(opts *MasterOpts) {
	listener, err := net.Listen("tcp", opts.GRPCAddr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterMasterServer(grpcServer, s)
	log.Printf("GRPC server listening on %v", listener.Addr())
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

// ReceiveHelloPingFromClient is called when a slave starts up and sends pings to master for its discovery
func (s *MasterServer) ReceiveHelloPingFromClient(ctx context.Context, req *pb.HelloPingRequest) (*pb.HelloPingResponse, error) {
	log.Printf("received message %v from slave %v", req.Message, req.SlaveName)
	s.slaveInfo[req.SlaveName] = &SlaveInfo{
		port:      req.SlavePort,
		slaveUUID: req.Slave_UUID,
	}
	return &pb.HelloPingResponse{Message: "Done!"}, nil
}
