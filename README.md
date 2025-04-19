# Структура библиотеки common-go

```
github.com/your-company/common-go/
├── config/
│   ├── config.go             // Базовая конфигурация и загрузка из env/файлов
│   └── default.go            // Установка значений по умолчанию
├── database/
│   ├── connection.go         // Подключение к базе данных
│   └── transaction.go        // Поддержка транзакций
├── errors/
│   ├── errors.go             // Стандартизированные ошибки
│   └── codes.go              // Коды ошибок
├── grpc/
│   ├── server.go             // Настройка gRPC сервера
│   ├── client.go             // Создание gRPC клиентов
│   ├── middleware/
│   │   ├── recovery.go       // Восстановление после паники
│   │   ├── logging.go        // Логирование запросов
│   │   └── auth.go           // Аутентификация
│   └── interceptors/
│       └── telemetry.go      // Интерцепторы для метрик/трейсинга
├── health/
│   ├── checker.go            // Интерфейс для проверки здоровья
│   ├── clients.go            // Клиенты для проверки Redis, RabbitMQ и т.д.
│   └── handler.go            // HTTP обработчик для endpoints /health и /readiness
├── http/
│   ├── server.go             // Настройка HTTP сервера
│   └── middleware/
│       ├── logger.go         // Middleware для логирования 
│       ├── metrics.go        // Middleware для метрик
│       ├── recovery.go       // Middleware для восстановления после паники
│       ├── cors.go           // Middleware для CORS
│       └── auth.go           // Middleware для аутентификации
├── logging/
│   ├── logger.go             // Логгер с уровнями и форматированием
│   └── context.go            // Работа с логгером в контексте
├── messaging/
│   ├── rabbitmq/
│   │   ├── publisher.go      // Издатель сообщений
│   │   ├── consumer.go       // Потребитель сообщений
│   │   └── connection.go     // Управление соединением
│   └── kafka/                // Дополнительно, если используете Kafka
├── metrics/
│   ├── prometheus.go         // Метрики Prometheus
│   └── exporter.go           // Экспорт метрик
├── ratelimit/
│   └── limiter.go            // Реализация ограничения частоты запросов
├── redis/
│   └── client.go             // Клиент Redis с повтором подключения
├── security/
│   ├── apikey.go             // Проверка API-ключа
│   └── jwt.go                // Работа с JWT токенами
├── telemetry/
│   ├── tracing.go            // Трассировка запросов (OpenTelemetry)
│   └── metrics.go            // Метрики приложения
├── validation/
│   └── validator.go          // Валидация данных
├── utils/
│   ├── env.go                // Утилиты для работы с переменными окружения
│   ├── pointers.go           // Утилиты для работы с указателями
│   └── time.go               // Утилиты для работы со временем
└── go.mod
```

## Описание компонентов

- **config**: Унифицированная система загрузки конфигурации из переменных окружения и файлов
- **database**: Общие функции для работы с базами данных
- **errors**: Стандартизированные ошибки и коды ошибок
- **grpc**: Настройка gRPC сервера и клиентов
- **health**: Компоненты для проверки здоровья сервисов
- **http**: HTTP сервер и middleware
- **logging**: Унифицированное логирование
- **messaging**: Работа с сообщениями (RabbitMQ, Kafka)
- **metrics**: Метрики Prometheus
- **ratelimit**: Ограничение частоты запросов
- **redis**: Клиент Redis
- **security**: Компоненты безопасности
- **telemetry**: Трассировка и мониторинг
- **validation**: Валидация данных
- **utils**: Утилитарные функции