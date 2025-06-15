package grpc_clients

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CallOptions опции для вызова gRPC методов
type CallOptions struct {
	Timeout    time.Duration
	Retries    int
	RetryDelay time.Duration
}

// DefaultCallOptions возвращает опции по умолчанию
func DefaultCallOptions() *CallOptions {
	return &CallOptions{
		Timeout:    30 * time.Second,
		Retries:    3,
		RetryDelay: 1 * time.Second,
	}
}

// GrpcCallWrapper обертка для выполнения gRPC вызовов с retry логикой
func GrpcCallWrapper[Req, Resp any](
	ctx context.Context,
	client *BaseServiceClient,
	methodName string,
	request Req,
	callFunc func(context.Context, Req, ...grpc.CallOption) (Resp, error),
	opts *CallOptions,
) (Resp, error) {
	var response Resp
	var lastErr error

	if opts == nil {
		opts = DefaultCallOptions()
	}

	for attempt := 0; attempt <= opts.Retries; attempt++ {
		// Создаем контекст с таймаутом для каждой попытки
		callCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		
		// Выполняем вызов
		resp, err := callFunc(callCtx, request)
		cancel()

		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Проверяем, стоит ли повторять запрос
		if !shouldRetry(err) || attempt == opts.Retries {
			break
		}

		// Ждем перед следующей попыткой
		select {
		case <-ctx.Done():
			return response, ctx.Err()
		case <-time.After(opts.RetryDelay * time.Duration(attempt+1)):
			// Exponential backoff
		}
	}

	return response, fmt.Errorf("все попытки вызова %s.%s исчерпаны: %w", 
		client.GetServiceName(), methodName, lastErr)
}

// shouldRetry определяет, стоит ли повторять запрос при данной ошибке
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Извлекаем gRPC статус
	st, ok := status.FromError(err)
	if !ok {
		return true // Неизвестная ошибка - повторяем
	}

	// Определяем коды ошибок, при которых имеет смысл повторить запрос
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

// ClientBuilder паттерн Builder для создания клиентов различных сервисов
type ClientBuilder struct {
	registry *ClientRegistry
	configs  map[string]*ServiceConfig
}

// NewClientBuilder создает новый builder для клиентов
func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{
		registry: NewClientRegistry(),
		configs:  make(map[string]*ServiceConfig),
	}
}

// AddService добавляет конфигурацию сервиса
func (b *ClientBuilder) AddService(name string, config *ServiceConfig) *ClientBuilder {
	b.configs[name] = config
	return b
}

// AddServiceWithDefaults добавляет сервис с базовой конфигурацией
func (b *ClientBuilder) AddServiceWithDefaults(name, address, port string) *ClientBuilder {
	config := &ServiceConfig{
		Address:     address,
		Port:        port,
		Timeout:     10 * time.Second,
		MaxRetries:  3,
		HealthCheck: true,
	}
	return b.AddService(name, config)
}

// Build создает реестр с зарегистрированными сервисами
func (b *ClientBuilder) Build() *ClientRegistry {
	for name, config := range b.configs {
		b.registry.RegisterService(name, config)
	}
	return b.registry
}

// ServiceNames константы для имен сервисов
const (
	ServiceAuth          = "auth-service"
	ServiceDevice        = "device-service"
	ServiceLocation      = "location-service"
	ServiceOrder         = "order-service"
	ServiceRepair        = "repair-service"
	ServiceServiceCenter = "service-center-service"
	ServiceReviews       = "reviews-service"
	ServicePrice         = "price-service"
	ServicePart          = "part-service"
	ServiceUser          = "user-service"
	ServiceAnalytics     = "analytics-service"
	ServiceNotification  = "notification-service"
	ServiceFinance       = "finance-service"
)

// CreateAllServicesRegistry создает реестр со всеми сервисами с настройками по умолчанию
func CreateAllServicesRegistry() *ClientRegistry {
	builder := NewClientBuilder()

	// Регистрируем все микросервисы
	services := map[string]string{
		ServiceAuth:          "50051",
		ServiceDevice:        "50052",
		ServiceLocation:      "50053",
		ServiceOrder:         "50054",
		ServiceRepair:        "50055",
		ServiceServiceCenter: "50056",
		ServiceReviews:       "50057",
		ServicePrice:         "50058",
		ServicePart:          "50059",
		ServiceUser:          "50060",
		ServiceAnalytics:     "50061",
		ServiceNotification:  "50062",
		ServiceFinance:       "50063",
	}

	for serviceName, port := range services {
		builder.AddServiceWithDefaults(serviceName, serviceName, port)
	}

	return builder.Build()
}