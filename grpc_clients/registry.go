package grpc_clients

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ServiceConfig содержит конфигурацию для подключения к сервису
type ServiceConfig struct {
	Address     string        `json:"address" yaml:"address"`
	Port        string        `json:"port" yaml:"port"`
	Timeout     time.Duration `json:"timeout" yaml:"timeout"`
	MaxRetries  int           `json:"max_retries" yaml:"max_retries"`
	HealthCheck bool          `json:"health_check" yaml:"health_check"`
}

// ClientRegistry централизованно управляет всеми gRPC клиентами
type ClientRegistry struct {
	connections map[string]*grpc.ClientConn
	configs     map[string]*ServiceConfig
	mu          sync.RWMutex
}

// ServiceClientInterface определяет общий интерфейс для всех клиентов
type ServiceClientInterface interface {
	Close() error
	GetConnection() *grpc.ClientConn
	IsHealthy(ctx context.Context) bool
}

// BaseServiceClient базовая реализация для всех клиентов
type BaseServiceClient struct {
	conn        *grpc.ClientConn
	serviceName string
	registry    *ClientRegistry
}

// NewClientRegistry создает новый реестр клиентов
func NewClientRegistry() *ClientRegistry {
	return &ClientRegistry{
		connections: make(map[string]*grpc.ClientConn),
		configs:     make(map[string]*ServiceConfig),
	}
}

// RegisterService регистрирует конфигурацию сервиса
func (r *ClientRegistry) RegisterService(serviceName string, config *ServiceConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Устанавливаем значения по умолчанию
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	
	r.configs[serviceName] = config
	log.Printf("Зарегистрирован сервис %s с адресом %s:%s", serviceName, config.Address, config.Port)
}

// GetConnection возвращает gRPC соединение для сервиса (создает при необходимости)
func (r *ClientRegistry) GetConnection(serviceName string) (*grpc.ClientConn, error) {
	r.mu.RLock()
	if conn, exists := r.connections[serviceName]; exists {
		r.mu.RUnlock()
		return conn, nil
	}
	r.mu.RUnlock()

	// Создаем новое соединение
	return r.createConnection(serviceName)
}

// createConnection создает новое gRPC соединение
func (r *ClientRegistry) createConnection(serviceName string) (*grpc.ClientConn, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Проверяем еще раз под блокировкой
	if conn, exists := r.connections[serviceName]; exists {
		return conn, nil
	}

	config, exists := r.configs[serviceName]
	if !exists {
		return nil, fmt.Errorf("конфигурация для сервиса %s не найдена", serviceName)
	}

	target := fmt.Sprintf("%s:%s", config.Address, config.Port)
	
	// Настройки keepalive для поддержания соединения
	kacp := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             time.Second,
		PermitWithoutStream: true,
	}

	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	// Опции подключения
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(kacp),
		grpc.WithBlock(), // Ждем подключения
	}

	log.Printf("Подключение к сервису %s по адресу %s", serviceName, target)

	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к сервису %s: %w", serviceName, err)
	}

	r.connections[serviceName] = conn
	log.Printf("Успешно подключен к сервису %s", serviceName)

	return conn, nil
}

// CreateClient создает клиент для указанного сервиса
func (r *ClientRegistry) CreateClient(serviceName string) (*BaseServiceClient, error) {
	conn, err := r.GetConnection(serviceName)
	if err != nil {
		return nil, err
	}

	return &BaseServiceClient{
		conn:        conn,
		serviceName: serviceName,
		registry:    r,
	}, nil
}

// Close закрывает соединение для сервиса
func (r *ClientRegistry) Close(serviceName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if conn, exists := r.connections[serviceName]; exists {
		delete(r.connections, serviceName)
		return conn.Close()
	}
	return nil
}

// CloseAll закрывает все соединения
func (r *ClientRegistry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for serviceName, conn := range r.connections {
		if err := conn.Close(); err != nil {
			log.Printf("Ошибка при закрытии соединения с сервисом %s: %v", serviceName, err)
		} else {
			log.Printf("Соединение с сервисом %s закрыто", serviceName)
		}
	}
	r.connections = make(map[string]*grpc.ClientConn)
}

// GetAllServices возвращает список всех зарегистрированных сервисов
func (r *ClientRegistry) GetAllServices() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]string, 0, len(r.configs))
	for serviceName := range r.configs {
		services = append(services, serviceName)
	}
	return services
}

// Реализация BaseServiceClient

// Close закрывает соединение клиента
func (c *BaseServiceClient) Close() error {
	return c.registry.Close(c.serviceName)
}

// GetConnection возвращает gRPC соединение
func (c *BaseServiceClient) GetConnection() *grpc.ClientConn {
	return c.conn
}

// IsHealthy проверяет состояние соединения
func (c *BaseServiceClient) IsHealthy(ctx context.Context) bool {
	if c.conn == nil {
		return false
	}
	
	// Проверяем состояние соединения
	state := c.conn.GetState()
	return state.String() == "READY" || state.String() == "IDLE"
}

// GetServiceName возвращает имя сервиса
func (c *BaseServiceClient) GetServiceName() string {
	return c.serviceName
}