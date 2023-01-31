package pkg

import (
	"context"
	"fmt"
	pb "github.com/golang-projects/master_slave/pkg/api"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/pprof"
	"sync"
	"time"
)

type OtherServerHostnames []string

type state uint32

const (
	FOLLOWER state = iota
	CANDIDATE
	LEADER
)

type RaftServer struct {
	pb.DiscoveryServer
	Name string
	// should be >= 1, as 0 is reserved for currentLeaderCandidateId to begin with
	candidateId uint32

	mu         *sync.RWMutex
	grpcServer *grpc.Server

	// HTTP server to serve pprof stats
	HTTPServer   *http.Server
	HTTPMux      *http.ServeMux
	HTTPListener net.Listener

	selfVotes     uint32
	electionTick  time.Duration
	heartbeatTick time.Duration

	electionRPCTimeout  time.Duration
	heartbeatRPCTimeout time.Duration

	otherServerHostnames                   OtherServerHostnames
	currentTerm                            uint32
	currentState                           state
	currentLeaderCandidateId               uint32
	termToVotedCandidateMap                map[uint32]uint32
	somebodyElseBecameLeaderForCurrentTerm chan uint32
	currentNodeBecameLeader                chan struct{}

	// Discovery clients for this server. Will be used to connect to other servers for requesting votes
	// and sending logs
	grpcDiscoveryClients []pb.DiscoveryClient
}

type ServerOpts struct {
	SelfGRPCAddr        string `json:"grpc_addr"`
	PprofHTTPServerHost string `json:"pprof_server_host"`
	Name                string `json:"name"`
	ServerId            uint32 `json:"candidate_id"`
}

func generateRandomTimeout() time.Duration {
	rand.Seed(time.Now().UnixNano())
	x := rand.Intn(200) + 100
	return time.Duration(x) * time.Millisecond
}

func NewRaftServer(opts *ServerOpts) (*RaftServer, error) {
	s := &RaftServer{
		Name:                                   opts.Name,
		mu:                                     &sync.RWMutex{},
		currentTerm:                            1,
		currentState:                           FOLLOWER,
		candidateId:                            opts.ServerId,
		HTTPMux:                                http.NewServeMux(),
		termToVotedCandidateMap:                map[uint32]uint32{},
		otherServerHostnames:                   []string{},
		grpcDiscoveryClients:                   []pb.DiscoveryClient{},
		electionTick:                           generateRandomTimeout(),
		heartbeatTick:                          5 * time.Millisecond,
		electionRPCTimeout:                     5 * time.Millisecond,
		heartbeatRPCTimeout:                    time.Millisecond,
		somebodyElseBecameLeaderForCurrentTerm: make(chan uint32),
		currentNodeBecameLeader:                make(chan struct{}),
	}

	for i := 1; i <= viper.GetInt("num"); i++ {
		hostname := fmt.Sprintf("localhost:908%v", i)
		if hostname != opts.SelfGRPCAddr {
			s.otherServerHostnames = append(s.otherServerHostnames, hostname)
		}
	}

	if err := s.initPprofServer(opts); err != nil {
		return nil, err
	}
	if err := s.initGRPCClients(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *RaftServer) Start(stop chan struct{}, opts *ServerOpts) error {
	log.Printf("Received start request for server %v", s.Name)
	go func() {
		log.Printf("starting HTTP service for pprof at %s", s.HTTPListener.Addr())
		if err := s.HTTPServer.Serve(s.HTTPListener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	go func() {
		if err := s.initGRPCServer(opts); err != nil {
			log.Fatalf("Failed to start GRPC server for server %v, err: %v", s.Name, err)
		}
	}()

	s.initHeartbeatsIfMaster()
	s.waitForShutdown(stop)
	return nil
}

func (s *RaftServer) initGRPCServer(opts *ServerOpts) error {
	listener, err := net.Listen("tcp", opts.SelfGRPCAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	s.grpcServer = grpc.NewServer()
	pb.RegisterDiscoveryServer(s.grpcServer, s)
	log.Printf("GRPC server listening on %v", listener.Addr())
	if err = s.grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}
	return nil
}

func (s *RaftServer) initGRPCClients() error {
	for _, host := range s.otherServerHostnames {
		conn, err := grpc.DialContext(context.Background(), host, grpc.WithInsecure())
		if err != nil {
			return err
		}
		s.grpcDiscoveryClients = append(s.grpcDiscoveryClients, pb.NewDiscoveryClient(conn))
	}
	return nil
}

func (s *RaftServer) initPprofServer(opts *ServerOpts) error {
	s.HTTPServer = &http.Server{
		Addr:    opts.PprofHTTPServerHost,
		Handler: s.HTTPMux,
	}

	listener, err := net.Listen("tcp", opts.PprofHTTPServerHost)
	if err != nil {
		return err
	}
	s.HTTPMux.HandleFunc("/debug/pprof/", pprof.Index)
	s.HTTPMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	s.HTTPMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	s.HTTPMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	s.HTTPMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	s.HTTPListener = listener
	return nil
}

func (s *RaftServer) initHeartbeatsIfMaster() {
	go func() {
		<-s.currentNodeBecameLeader
		s.sendHeartbeat()
		for {
			select {
			case <-time.After(s.heartbeatTick):
				log.Printf("Sending heartbeats to all servers from leader %v", s.Name)
				s.sendHeartbeat()
			case <-s.somebodyElseBecameLeaderForCurrentTerm:
				break
			}
		}
	}()
}

func (s *RaftServer) sendHeartbeat() {
	var wg sync.WaitGroup
	for _, client := range s.grpcDiscoveryClients {
		wg.Add(1)
		go func(client pb.DiscoveryClient) {
			defer wg.Done()
			log.Printf("Calling AppendEntry RPC for all hosts from %v, which is now the LEADER", s.Name)
			ctx, cancel := context.WithTimeout(context.Background(), s.heartbeatRPCTimeout)
			defer cancel()
			req := &pb.AppendEntriesRequest{
				Term:              s.currentTerm,
				LeaderCandidateId: s.candidateId,
				// Empty log entries mean this is just a heartbeat
				Entries: []*pb.LogEntriesToAppend{},
			}
			resp, err := client.AppendEntries(ctx, req)
			if err != nil {
				log.Fatalf("Failed to send AppendEntries request from %v, err: %v", s.Name, err)
			}
			log.Printf("Current config of server %v: %v", resp.Name, resp)
			if !resp.Success {
				// Some other node's term is higher than this node, reject the current node as leader.
				// Leader election process will be restarted by the other node.
			}
		}(client)
	}
	wg.Wait()
}

func (s *RaftServer) SetTimeoutAndStartVoteRequest() {
	select {
	case <-time.After(s.electionTick):
		s.mu.Lock()
		// If somebody has already been chosen as a leader, then there is no need to start election
		if s.currentLeaderCandidateId != 0 {
			return
		}
		s.electionTick = generateRandomTimeout()
		s.mu.Unlock()

		go s.SetTimeoutAndStartVoteRequest()

		// increment current term, vote for self and request vote from others
		s.mu.Lock()
		s.currentTerm++
		s.selfVotes++
		s.currentState = CANDIDATE
		s.mu.Unlock()

		var wg sync.WaitGroup
		for _, client := range s.grpcDiscoveryClients {
			wg.Add(1)
			go func(client pb.DiscoveryClient) {
				defer wg.Done()
				log.Printf("Sending request vote request to all hosts from %v", s.Name)
				ctx, cancel := context.WithTimeout(context.Background(), s.electionRPCTimeout)
				defer cancel()
				req := &pb.RequestVoteRequest{
					Term:        s.currentTerm,
					CandidateId: s.candidateId,
				}
				resp, err := client.RequestVote(ctx, req)
				if err != nil {
					log.Fatalf("Failed to request vote from %v", s.Name)
				}
				s.mu.Lock()
				defer s.mu.Unlock()
				if resp.VoteGranted {
					s.selfVotes++
					if s.currentState == LEADER {
						return
					}
					if int(s.selfVotes) > len(s.grpcDiscoveryClients)/2 {
						s.currentState = LEADER
						s.currentLeaderCandidateId = s.candidateId
						s.currentNodeBecameLeader <- struct{}{}
					}
				}
				log.Printf("Response after requesting vote from %v: %v", s.Name, resp)
			}(client)
		}
		wg.Wait()

	// Somebody else has become the leader, update currentLeaderCandidateId and return
	case leaderCandidateId := <-s.somebodyElseBecameLeaderForCurrentTerm:
		s.mu.Lock()
		defer s.mu.Unlock()
		s.currentLeaderCandidateId = leaderCandidateId
		s.currentState = FOLLOWER
		return
	}
}

func (s *RaftServer) RequestVote(ctx context.Context, in *pb.RequestVoteRequest) (*pb.RequestVoteResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := &pb.RequestVoteResponse{
		CurrentTerm: s.currentTerm,
		VoteGranted: false,
	}

	if val, ok := s.termToVotedCandidateMap[in.Term]; ok || val == in.CandidateId {
		// If vote for this term has already been cast to some other node or the same node which sent the request
		// then ignore
		return resp, nil
	} else {
		if in.Term >= s.currentTerm {
			s.currentTerm = in.Term
			s.termToVotedCandidateMap[in.Term] = in.CandidateId
			resp.VoteGranted = true
			return resp, nil
		} else {
			return resp, nil
		}
	}
}

func (s *RaftServer) AppendEntries(ctx context.Context, in *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resp := &pb.AppendEntriesResponse{
		Term:         s.currentTerm,
		Name:         s.Name,
		CurrentState: uint32(s.currentState),
		Success:      true,
	}

	// If leader's term < current node's term, reject the leader's request to become leader. This case happens when
	// current node missed some heartbeat from the leader and hence started the election process after promoting itself
	// to CANDIDATE state.
	if in.Term < s.currentTerm {
		resp.Success = false
		return resp, nil
	}

	// If a leader has already been elected for the current term, then simply return success = true. This case happens when
	// leader is sending repeated heartbeats after it became leader.
	if s.currentLeaderCandidateId != 0 {
		return resp, nil
	}

	// Else accept the leader, inform the somebodyElseBecameLeaderForCurrentTerm channel,
	// which sets currentLeaderCandidateId to the leader's candidateId and moves the current node
	// back to follower state.
	s.somebodyElseBecameLeaderForCurrentTerm <- in.LeaderCandidateId
	return resp, nil
}

func (s *RaftServer) waitForShutdown(stop chan struct{}) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		<-stop
		defer wg.Done()
		if s.HTTPServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := s.HTTPServer.Shutdown(ctx)
			if err != nil {
				log.Fatalf("Failed to shutdown http server for server %v, err: %v", s.Name, err)
			}
		}
		if s.grpcServer != nil {
			s.grpcServer.Stop()
		}
	}()
	wg.Wait()
}
