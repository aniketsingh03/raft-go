package pkg

import (
	"context"
	pb "github.com/golang-projects/master_slave/pkg/api"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"sync"
	"time"
)

type Slave struct {
	Name string
	UUID string

	// HTTP server to serve pprof stats
	HTTPServer   *http.Server
	HTTPMux      *http.ServeMux
	HTTPListener net.Listener

	// Discovery client for slave. Will be used to register this slave with master
	slaveDiscoveryClient pb.DiscoveryClient
}

type SlaveOpts struct {
	MasterHost      string `json:"master"`
	PprofServerHost string `json:"pprof"`
	Name            string `json:"name"`
	MessageToMaster string `json:"message"`
}

func NewSlave(opts *SlaveOpts) (*Slave, error) {
	s := &Slave{
		HTTPMux: http.NewServeMux(),
	}
	if err := s.initPprofServer(opts); err != nil {
		return nil, err
	}
	if err := s.initSlaveDiscovery(opts); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Slave) Start(stop chan struct{}, opts *SlaveOpts) error {
	log.Printf("Received start request for slave %v", opts.Name)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		log.Printf("starting HTTP service for pprof at %s", s.HTTPListener.Addr())
		if err := s.HTTPServer.Serve(s.HTTPListener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		log.Print("sending registration request to master server")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if s.slaveDiscoveryClient != nil {
			req := &pb.RegisterSlaveRequest{
				Message:             opts.MessageToMaster,
				SlaveHttpServerHost: opts.PprofServerHost,
				SlaveName:           opts.Name,
			}
			resp, err := s.slaveDiscoveryClient.RegisterSlave(ctx, req)
			if err != nil {
				log.Fatalf("Failed to register slave with master")
			}
			log.Printf("Response after registering slave %v: %v", opts.Name, resp)
		}
	}()
	s.waitForShutdown(stop)
	wg.Wait()
	return nil
}

func (s *Slave) waitForShutdown(stop chan struct{}) {
	go func() {
		<-stop
		if s.HTTPServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := s.HTTPServer.Shutdown(ctx)
			if err != nil {
				log.Fatalf("Failed to shutdown http server for slave, err: %v", err)
			}
		}
	}()
}
func (s *Slave) initSlaveDiscovery(opts *SlaveOpts) error {
	conn, err := grpc.DialContext(context.Background(), opts.MasterHost, grpc.WithInsecure())
	if err != nil {
		return err
	}
	s.slaveDiscoveryClient = pb.NewDiscoveryClient(conn)
	return nil
}

func (s *Slave) initPprofServer(opts *SlaveOpts) error {
	s.HTTPServer = &http.Server{
		Addr:    opts.PprofServerHost,
		Handler: s.HTTPMux,
	}

	listener, err := net.Listen("tcp", opts.PprofServerHost)
	if err != nil {
		return err
	}
	s.HTTPMux.HandleFunc("/debug/pprof/", pprof.Index)
	s.HTTPMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	s.HTTPMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	s.HTTPMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	s.HTTPMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	s.HTTPListener = listener
	log.Printf("set http listener")
	return nil
}
