package grpc_clients

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

// Config представляет конфигурацию для клиентов
type Config struct {
	Services map[string]ServiceConfigBase `yaml:"services" json:"services"`
}

// ServiceConfigBase конфигурация отдельного сервиса
type ServiceConfigBase struct {
	Address     string        `yaml:"address" json:"address"`
	Port        string        `yaml:"port" json:"port"`
	Timeout     time.Duration `yaml:"timeout" json:"timeout"`
	MaxRetries  int           `yaml:"max_retries" json:"max_retries"`
	HealthCheck bool          `yaml:"health_check" json:"health_check"`
}

// BaseClient базовый gRPC клиент
type BaseClient struct {
	Conn        *grpc.ClientConn
	Config      *Config
	ServiceName string
}

// ClientOptions опции для создания клиента
type ClientOptions struct {
	ServiceName     string
	ServiceURLKey   string
	DefaultPort     string
	ConnectTimeout  time.Duration
	ExtraDialOption []grpc.DialOption
	OnConnected     func(*grpc.ClientConn)
	OnError         func(error) error
	EnableLogging   bool
}

// OptionFunc функциональная опция
type OptionFunc func(*ClientOptions)

// WithDialOptions добавляет дополнительные опции для grpc.Dial
func WithDialOptions(opts ...grpc.DialOption) OptionFunc {
	return func(o *ClientOptions) {
		o.ExtraDialOption = append(o.ExtraDialOption, opts...)
	}
}

// WithConnectTimeout устанавливает таймаут подключения
func WithConnectTimeout(timeout time.Duration) OptionFunc {
	return func(o *ClientOptions) {
		o.ConnectTimeout = timeout
	}
}

// WithLogging включает или выключает логирование
func WithLogging(enable bool) OptionFunc {
	return func(o *ClientOptions) {
		o.EnableLogging = enable
	}
}

// DefaultOptions возвращает опции по умолчанию
func DefaultOptions(serviceName, serviceURLKey, defaultPort string) ClientOptions {
	return ClientOptions{
		ServiceName:    serviceName,
		ServiceURLKey:  serviceURLKey,
		DefaultPort:    defaultPort,
		ConnectTimeout: 5 * time.Second,
		EnableLogging:  true,
	}
}

// NewBaseClient создает новый базовый клиент
func NewBaseClient(cfg *Config, options ClientOptions) (*BaseClient, error) {
	// Получаем URL сервиса из конфигурации
	serviceURL := fmt.Sprintf("%s:%s", options.ServiceName, options.DefaultPort)

	// Создаем контекст с таймаутом для подключения
	ctx, cancel := context.WithTimeout(context.Background(), options.ConnectTimeout)
	defer cancel()

	// Настраиваем опции подключения с keepalive
	kacp := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             time.Second,
		PermitWithoutStream: true,
	}

	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(kacp),
		grpc.WithBlock(),
	}

	// Добавляем дополнительные опции
	if len(options.ExtraDialOption) > 0 {
		dialOptions = append(dialOptions, options.ExtraDialOption...)
	}

	if options.EnableLogging {
		log.Printf("Подключение к сервису %s по адресу %s...", options.ServiceName, serviceURL)
	}

	// Инициализируем соединение
	conn, err := grpc.DialContext(ctx, serviceURL, dialOptions...)
	if err != nil {
		if options.OnError != nil {
			return nil, options.OnError(err)
		}
		return nil, fmt.Errorf("ошибка подключения к %s: %w", options.ServiceName, err)
	}

	if options.EnableLogging {
		log.Printf("Успешное подключение к сервису %s", options.ServiceName)
	}

	// Вызываем callback после успешного подключения
	if options.OnConnected != nil {
		options.OnConnected(conn)
	}

	return &BaseClient{
		Conn:        conn,
		Config:      cfg,
		ServiceName: options.ServiceName,
	}, nil
}

// NewBaseClientWithOptions создает базовый клиент с функциональными опциями
func NewBaseClientWithOptions(cfg *Config, serviceName, serviceURLKey, defaultPort string, opts ...OptionFunc) (*BaseClient, error) {
	options := DefaultOptions(serviceName, serviceURLKey, defaultPort)

	// Применяем функциональные опции
	for _, opt := range opts {
		opt(&options)
	}

	return NewBaseClient(cfg, options)
}

// Close закрывает соединение
func (c *BaseClient) Close() error {
	if c.Conn != nil {
		return c.Conn.Close()
	}
	return nil
}

// MeasureCall выполняет gRPC запрос с измерением времени и retry логикой
func MeasureCall[Req any, Resp any](
	ctx context.Context,
	serviceName, methodName string,
	request Req,
	call func(context.Context, Req, ...grpc.CallOption) (Resp, error),
	opts ...grpc.CallOption,
) (Resp, error) {
	var emptyResp Resp

	start := time.Now()
	
	// Логируем запрос
	log.Printf("gRPC call: %s.%s", serviceName, methodName)

	resp, err := call(ctx, request, opts...)
	duration := time.Since(start)

	if err != nil {
		log.Printf("gRPC error in %s.%s: %v (duration: %v)", serviceName, methodName, err, duration)
		return emptyResp, fmt.Errorf("сервис %s недоступен: %w", serviceName, err)
	}

	log.Printf("gRPC success: %s.%s (duration: %v)", serviceName, methodName, duration)
	return resp, nil
}

// shouldRetryCall определяет, стоит ли повторять запрос
func shouldRetryCall(err error) bool {
	if err == nil {
		return false
	}

	st, ok := status.FromError(err)
	if !ok {
		return true
	}

	switch st.Code() {
	case codes.DeadlineExceeded,
		codes.Unavailable,
		codes.ResourceExhausted,
		codes.Aborted,
		codes.Internal:
		return true
	default:
		return false
	}
}