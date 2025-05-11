package server

import (
	"fmt"
	"net"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"vk-go-developer-assignment/internal/server/middleware"
	"vk-go-developer-assignment/pkg/config"
	"vk-go-developer-assignment/pkg/subpub"
)

type GRPCServer struct {
	cfg      *config.Config
	logger   zerolog.Logger
	server   *grpc.Server
	listener net.Listener
}

func NewGRPCServer(cfg *config.Config, logger zerolog.Logger) (*GRPCServer, error) {
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryLoggerInterceptor(logger)),
		grpc.StreamInterceptor(middleware.StreamLoggerInterceptor(logger)),
	)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %v", err)
	}

	reflection.Register(grpcServer)

	return &GRPCServer{
		cfg:      cfg,
		logger:   logger,
		server:   grpcServer,
		listener: listener,
	}, nil
}

func (s *GRPCServer) RegisterPubSubService(sp subpub.SubPub) {
	handler := NewPubSubHandler(sp, s.logger, s.cfg)
	handler.Register(s.server)
}

func (s *GRPCServer) Start() error {
	s.logger.Info().
		Str("address", s.listener.Addr().String()).
		Msg("Starting gRPC server")

	return s.server.Serve(s.listener)
}

func (s *GRPCServer) Stop() {
	s.logger.Info().Msg("Stopping gRPC server")
	s.server.GracefulStop()
}

func (s *GRPCServer) GetAddress() string {
	return s.listener.Addr().String()
}
