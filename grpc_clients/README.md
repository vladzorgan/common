# Централизованная система gRPC клиентов

Эта библиотека предоставляет унифицированную систему для работы с gRPC клиентами во всех микросервисах проекта.

## Основные преимущества

1. **Централизованное управление соединениями** - все настройки подключений в одном месте
2. **Повторное использование кода** - избежание дублирования клиентского кода
3. **Единообразная обработка ошибок** - стандартизированная логика retry и логирования
4. **Типобезопасность** - поддержка типизированных proto-сгенерированных клиентов

## Архитектура

### Базовые компоненты

- `BaseClient` - базовый gRPC клиент с управлением соединениями
- `Config` - конфигурация для всех сервисов
- `MeasureCall` - обертка для выполнения gRPC вызовов с метриками

### Клиенты сервисов

- `LocationClient` - базовый клиент для location-service
- Аналогично для других сервисов (device, order, etc.)

## Использование

### В API Gateway

```go
// 1. Создаем конфигурацию
config := grpc_clients.DefaultConfig()

// 2. Создаем фабрику клиентов
factory := grpc_clients.NewClientFactory(config)

// 3. Создаем типизированный клиент
type APIGatewayLocationClient struct {
    *grpc_clients.LocationClient
    client locationpb.LocationServiceClient
}

func NewAPIGatewayLocationClient(cfg *grpc_clients.Config) (*APIGatewayLocationClient, error) {
    baseClient, err := grpc_clients.NewLocationClient(cfg)
    if err != nil {
        return nil, err
    }

    protoClient := locationpb.NewLocationServiceClient(baseClient.Conn)

    return &APIGatewayLocationClient{
        LocationClient: baseClient,
        client:         protoClient,
    }, nil
}

// 4. Реализуем методы с типизацией
func (c *APIGatewayLocationClient) GetRegion(ctx context.Context, id uint32) (*locationpb.RegionResponse, error) {
    request := &locationpb.GetRegionRequest{Id: id}
    return grpc_clients.MeasureCall(ctx, grpc_clients.LocationServiceName, "GetRegion", request, c.client.GetRegion)
}
```

### В микросервисе

```go
// В любом микросервисе создаем только нужные клиенты
config := &grpc_clients.Config{
    Services: map[string]grpc_clients.ServiceConfigBase{
        "location-service": {
            Address: "location-service",
            Port:    "50053",
        },
    },
}

// Создаем типизированный клиент
type MicroserviceLocationClient struct {
    *grpc_clients.LocationClient
    client locationpb.LocationServiceClient
}

func NewMicroserviceLocationClient(cfg *grpc_clients.Config) (*MicroserviceLocationClient, error) {
    baseClient, err := grpc_clients.NewLocationClient(cfg)
    if err != nil {
        return nil, err
    }

    protoClient := locationpb.NewLocationServiceClient(baseClient.Conn)

    return &MicroserviceLocationClient{
        LocationClient: baseClient,
        client:         protoClient,
    }, nil
}
```

## Структура файлов

```
backend/common/grpc_clients/
├── base_client.go          # Базовый клиент и конфигурация
├── clients.go              # Общие утилиты и wrapper'ы (устаревший)
├── location_client.go      # Клиент для location-service
├── registry.go             # Реестр клиентов (устаревший)
└── README.md              # Документация
```

## Миграция существующих клиентов

### Для API Gateway

1. Замените существующие клиенты на новые:

```go
// Старый код
locationClient := location.NewClient(cfg)

// Новый код  
locationClient := NewAPIGatewayLocationClient(grpcConfig)
```

2. Обновите вызовы методов:

```go
// Старый код
response, err := locationClient.GetRegion(ctx, id)

// Новый код (остается тем же)
response, err := locationClient.GetRegion(ctx, id)
```

### Для микросервисов

1. Создайте типизированный клиент для нужного сервиса
2. Замените прямые gRPC вызовы на методы клиента
3. Удалите дублированный код подключения

## Добавление нового сервиса

1. Добавьте константы в `location_client.go` (или создайте новый файл):

```go
const (
    NewServiceName   = "new-service"
    NewServiceURLKey = "new-service"  
    NewDefaultPort   = "50064"
)
```

2. Создайте базовый клиент:

```go
type NewServiceClient struct {
    *BaseClient
}

func NewNewServiceClient(cfg *Config) (*NewServiceClient, error) {
    options := DefaultOptions(NewServiceName, NewServiceURLKey, NewDefaultPort)
    baseClient, err := NewBaseClient(cfg, options)
    if err != nil {
        return nil, err
    }
    return &NewServiceClient{BaseClient: baseClient}, nil
}
```

3. В проектах создайте типизированные обертки аналогично примерам выше.

## Конфигурация

Конфигурация задается через структуру `Config`:

```yaml
services:
  location-service:
    address: "location-service"
    port: "50053"
    timeout: "10s"
    max_retries: 3
    health_check: true
```

Или программно:

```go
config := &grpc_clients.Config{
    Services: map[string]grpc_clients.ServiceConfigBase{
        "location-service": {
            Address: "location-service",
            Port:    "50053",
            Timeout: 10 * time.Second,
            MaxRetries: 3,
            HealthCheck: true,
        },
    },
}
```