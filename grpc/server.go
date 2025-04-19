// Package grpc предоставляет унифицированный интерфейс для работы с gRPC сервером
package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/vladzorgan/common/config"
	"github.com/vladzorgan/common/grpc/interceptors"
	"github.com/vladzorgan/common/logging"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// Server представляет gRPC сервер
type Server struct {
	server     *grpc.Server
	config     *config.BaseConfig
	logger     logging.Logger
	healthSrv  *health.Server
	serviceMap map[string]struct{}
}

// ServerOptions содержит опции для создания gRPC сервера
type ServerOptions struct {
	// Включить отражение для gRPC
	EnableReflection bool
	// Включить проверку здоровья
	EnableHealth bool
	// Максимальный размер сообщений для отправки
	MaxSendMsgSize int
	// Максимальный размер сообщений для приема
	MaxRecvMsgSize int
	// Параметры keepalive
	KeepaliveParams keepalive.ServerParameters
	// Политика keepalive
	KeepalivePolicy keepalive.EnforcementPolicy
	// Дополнительные опции сервера
	AdditionalOptions []grpc.ServerOption
}

// DefaultServerOptions возвращает опции по умолчанию
func DefaultServerOptions(cfg *config.BaseConfig) *ServerOptions {
	return &ServerOptions{
		EnableReflection: cfg.EnableReflection,
		EnableHealth:     true,
		MaxSendMsgSize:   cfg.GRPCMaxSendMsgSize,
		MaxRecvMsgSize:   cfg.GRPCMaxRecvMsgSize,
		KeepaliveParams: keepalive.ServerParameters{
			MaxConnectionIdle:     cfg.GRPCKeepAliveTime,
			MaxConnectionAge:      cfg.GRPCKeepAliveTime * 2,
			MaxConnectionAgeGrace: cfg.GRPCKeepAliveTime,
			Time:                  cfg.GRPCKeepAliveTime,
			Timeout:               cfg.GRPCKeepAliveTimeout,
		},
		KeepalivePolicy: keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		},
		AdditionalOptions: []grpc.ServerOption{
			grpc.Creds(insecure.NewCredentials()), // Для разработки
		},
	}
}

// NewServer создает новый gRPC сервер
func NewServer(cfg *config.BaseConfig, logger logging.Logger, options *ServerOptions) *Server {
	if logger == nil {
		logger = logging.NewLogger()
	}

	if options == nil {
		options = DefaultServerOptions(cfg)
	}

	// Базовые опции сервера
	serverOptions := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(options.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(options.MaxSendMsgSize),
		grpc.KeepaliveParams(options.KeepaliveParams),
		grpc.KeepaliveEnforcementPolicy(options.KeepalivePolicy),
	}

	// Добавляем интерцепторы для унарных запросов
	serverOptions = append(serverOptions, grpc.UnaryInterceptor(
		interceptors.ChainUnaryInterceptors(
			interceptors.LoggingUnaryInterceptor(logger),
			interceptors.RecoveryUnaryInterceptor(logger),
			interceptors.MetricsUnaryInterceptor(cfg.ServicePrefix),
		),
	))

	// Добавляем интерцепторы для потоковых запросов
	serverOptions = append(serverOptions, grpc.StreamInterceptor(
		interceptors.ChainStreamInterceptors(
			interceptors.LoggingStreamInterceptor(logger),
			interceptors.RecoveryStreamInterceptor(logger),
			interceptors.MetricsStreamInterceptor(cfg.ServicePrefix),
		),
	))

	// Добавляем дополнительные опции
	serverOptions = append(serverOptions, options.AdditionalOptions...)

	// Создаем gRPC сервер
	grpcServer := grpc.NewServer(serverOptions...)

	// Создаем сервер
	server := &Server{
		server:     grpcServer,
		config:     cfg,
		logger:     logger,
		serviceMap: make(map[string]struct{}),
	}

	// Включаем отражение для gRPC, если нужно
	if options.EnableReflection {
		reflection.Register(grpcServer)
	}

	// Включаем проверку здоровья, если нужно
	if options.EnableHealth {
		server.healthSrv = health.NewServer()
		healthpb.RegisterHealthServer(grpcServer, server.healthSrv)
	}

	return server
}

// RegisterService регистрирует gRPC сервис
func (s *Server) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.server.RegisterService(desc, impl)

	// Регистрируем сервис для проверки здоровья
	if s.healthSrv != nil {
		s.healthSrv.SetServingStatus(desc.ServiceName, healthpb.HealthCheckResponse_SERVING)
		s.serviceMap[desc.ServiceName] = struct{}{}
	}
}

// Start запускает gRPC сервер
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", s.config.GRPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %v", s.config.GRPCPort, err)
	}

	s.logger.Info("gRPC server is starting on port %s", s.config.GRPCPort)

	// Логируем зарегистрированные сервисы
	for serviceName := range s.serviceMap {
		s.logger.Info("Registered gRPC service: %s", serviceName)
	}

	if err := s.server.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}

	return nil
}

// StartAsync запускает gRPC сервер в отдельной горутине
func (s *Server) StartAsync() {
	go func() {
		if err := s.Start(); err != nil {
			s.logger.Error("gRPC server failed: %v", err)
		}
	}()
}

// Stop останавливает gRPC сервер
func (s *Server) Stop() {
	s.logger.Info("Stopping gRPC server...")
	s.server.GracefulStop()
	s.logger.Info("gRPC server stopped")
}

// StopWithContext останавливает gRPC сервер с контекстом
func (s *Server) StopWithContext(ctx context.Context) {
	s.logger.Info("Stopping gRPC server with context...")

	// Создаем канал для сигнализации о завершении GracefulStop
	stopped := make(chan struct{})

	go func() {
		s.server.GracefulStop()
		close(stopped)
	}()

	// Ждем, пока сервер остановится или истечет время ожидания
	select {
	case <-stopped:
		s.logger.Info("gRPC server stopped gracefully")
	case <-ctx.Done():
		s.logger.Warn("Context deadline exceeded, forcing gRPC server to stop")
		s.server.Stop()
		s.logger.Info("gRPC server stopped forcefully")
	}
}

// Server возвращает экземпляр grpc.Server
func (s *Server) Server() *grpc.Server {
	return s.server
}

// SetServiceStatus устанавливает статус сервиса для проверки здоровья
func (s *Server) SetServiceStatus(serviceName string, status healthpb.HealthCheckResponse_ServingStatus) {
	if s.healthSrv != nil {
		s.healthSrv.SetServingStatus(serviceName, status)
	}
}
