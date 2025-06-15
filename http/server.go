// Package http предоставляет унифицированный интерфейс для работы с HTTP сервером
package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/vladzorgan/common/config"
	"github.com/vladzorgan/common/health"
	"github.com/vladzorgan/common/http/middleware"
	"github.com/vladzorgan/common/logging"
	"github.com/vladzorgan/common/metrics"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server представляет HTTP сервер
type Server struct {
	router      *gin.Engine
	httpServer  *http.Server
	cfg         *config.BaseConfig
	logger      logging.Logger
	healthCheck *health.Checker
}

// ServerOptions содержит опции для создания HTTP сервера
type ServerOptions struct {
	EnableCORS     bool
	EnableMetrics  bool
	EnableHealth   bool
	EnableSwagger  bool
	TrustedProxies []string
	SkipLogPaths   []string
}

// DefaultServerOptions возвращает опции по умолчанию
func DefaultServerOptions() *ServerOptions {
	return &ServerOptions{
		EnableCORS:     true,
		EnableMetrics:  true,
		EnableHealth:   true,
		EnableSwagger:  true,
		TrustedProxies: []string{"127.0.0.1"},
		SkipLogPaths:   []string{"/metrics", "/api/health"},
	}
}

// NewServer создает новый HTTP сервер
func NewServer(cfg *config.BaseConfig, logger logging.Logger, options *ServerOptions) *Server {
	if logger == nil {
		logger = logging.NewLogger()
	}

	if options == nil {
		options = DefaultServerOptions()
	}

	// Устанавливаем режим работы Gin
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Создаем экземпляр роутера
	router := gin.New()

	// Настраиваем middleware
	router.Use(gin.Recovery())
	router.Use(middleware.LoggerWithSkipPaths(logger, options.SkipLogPaths))
	router.Use(middleware.RequestID())

	// Добавляем middleware для метрик
	if options.EnableMetrics {
		router.Use(metrics.MetricsMiddleware())
	}

	// Настраиваем CORS
	if options.EnableCORS {
		router.Use(cors.New(cors.Config{
			AllowOrigins:     cfg.CorsOrigins,
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With", "X-Internal-API-Key"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}

	// Настраиваем доверенные прокси
	if len(options.TrustedProxies) > 0 {
		router.SetTrustedProxies(options.TrustedProxies)
	}

	// Создаем экземпляр HTTP сервера
	server := &Server{
		router: router,
		httpServer: &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      router,
			ReadTimeout:  time.Duration(cfg.TimeoutSeconds) * time.Second,
			WriteTimeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		cfg:    cfg,
		logger: logger,
	}

	// Добавляем эндпоинт метрик
	if options.EnableMetrics {
		router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	// Добавляем эндпоинты для проверки здоровья
	if options.EnableHealth {
		server.healthCheck = health.NewChecker(cfg.ServiceName, cfg.ServicePrefix, cfg.Version)
		healthHandler := health.NewHTTPHandler(server.healthCheck)
		healthHandler.RegisterHandlers(router)
	}

	return server
}

// Router возвращает экземпляр Gin роутера
func (s *Server) Router() *gin.Engine {
	return s.router
}

// HealthChecker возвращает экземпляр проверки здоровья
func (s *Server) HealthChecker() *health.Checker {
	return s.healthCheck
}

// RegisterHealthComponent регистрирует компонент для проверки здоровья
func (s *Server) RegisterHealthComponent(component health.Component) {
	if s.healthCheck != nil {
		s.healthCheck.RegisterComponent(component)
	}
}

// Start запускает HTTP сервер
func (s *Server) Start() error {
	s.logger.Info("Starting HTTP server on port %s", s.cfg.Port)

	// Запускаем сервер
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %v", err)
	}

	return nil
}

// StartAsync запускает HTTP сервер в отдельной горутине
func (s *Server) StartAsync() {
	go func() {
		if err := s.Start(); err != nil {
			s.logger.Error("HTTP server failed: %v", err)
		}
	}()
}

// Shutdown останавливает HTTP сервер
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server...")

	// Создаем контекст с таймаутом, если не задан
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
	}

	// Останавливаем HTTP сервер
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP server shutdown failed: %v", err)
	}

	s.logger.Info("HTTP server stopped")
	return nil
}

// Group создает новую группу маршрутов
func (s *Server) Group(relativePath string, handlers ...gin.HandlerFunc) *gin.RouterGroup {
	return s.router.Group(relativePath, handlers...)
}

// GET регистрирует обработчик GET запросов
func (s *Server) GET(relativePath string, handlers ...gin.HandlerFunc) {
	s.router.GET(relativePath, handlers...)
}

// POST регистрирует обработчик POST запросов
func (s *Server) POST(relativePath string, handlers ...gin.HandlerFunc) {
	s.router.POST(relativePath, handlers...)
}

// PUT регистрирует обработчик PUT запросов
func (s *Server) PUT(relativePath string, handlers ...gin.HandlerFunc) {
	s.router.PUT(relativePath, handlers...)
}

// DELETE регистрирует обработчик DELETE запросов
func (s *Server) DELETE(relativePath string, handlers ...gin.HandlerFunc) {
	s.router.DELETE(relativePath, handlers...)
}

// PATCH регистрирует обработчик PATCH запросов
func (s *Server) PATCH(relativePath string, handlers ...gin.HandlerFunc) {
	s.router.PATCH(relativePath, handlers...)
}

// OPTIONS регистрирует обработчик OPTIONS запросов
func (s *Server) OPTIONS(relativePath string, handlers ...gin.HandlerFunc) {
	s.router.OPTIONS(relativePath, handlers...)
}

// HEAD регистрирует обработчик HEAD запросов
func (s *Server) HEAD(relativePath string, handlers ...gin.HandlerFunc) {
	s.router.HEAD(relativePath, handlers...)
}

// Any регистрирует обработчик для всех типов запросов
func (s *Server) Any(relativePath string, handlers ...gin.HandlerFunc) {
	s.router.Any(relativePath, handlers...)
}
