// Package config предоставляет универсальный интерфейс для загрузки конфигурации сервиса
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// BaseConfig содержит основные настройки, общие для всех сервисов
type BaseConfig struct {
	// Основные настройки приложения
	ServiceName    string
	ServicePrefix  string
	URLPrefix      string
	Version        string
	Port           string
	Env            string
	LogLevel       string
	TimeoutSeconds int

	// Настройки CORS
	CorsOrigins []string

	// Настройки базы данных
	DatabaseURL string

	// Настройки RabbitMQ
	RabbitMQURL string

	// Настройки Redis
	RedisURL      string
	RedisPassword string
	RedisDB       int

	// Настройки безопасности
	InternalAPIKey string

	// Настройки пагинации
	DefaultPaginationLimit int

	// Настройки rate limiting
	RateLimitRequests int
	RateLimitInterval time.Duration

	// Настройки gRPC сервера
	GRPCPort             string
	GRPCMaxRecvMsgSize   int
	GRPCMaxSendMsgSize   int
	GRPCKeepAliveTime    time.Duration
	GRPCKeepAliveTimeout time.Duration
	EnableReflection     bool
}

// LoadBaseConfig загружает базовую конфигурацию из переменных окружения
func LoadBaseConfig() (*BaseConfig, error) {
	// Устанавливаем значения по умолчанию
	config := &BaseConfig{
		ServiceName:    getEnv("SERVICE_NAME", "Microservice"),
		ServicePrefix:  getEnv("SERVICE_PREFIX", "microservice"),
		URLPrefix:      getEnv("URL_PREFIX", "api"),
		Version:        getEnv("VERSION", "0.1.0"),
		Port:           getEnv("PORT", "8080"),
		Env:            getEnv("ENV", "development"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		TimeoutSeconds: getEnvAsInt("TIMEOUT_SECONDS", 30),

		// CORS
		CorsOrigins: strings.Split(getEnv("CORS_ORIGINS", "*"), ","),

		// База данных
		DatabaseURL: getEnv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/service_db?sslmode=disable"),

		// RabbitMQ
		RabbitMQURL: getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),

		// Redis
		RedisURL:      getEnv("REDIS_URL", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvAsInt("REDIS_DB", 0),

		// Безопасность
		InternalAPIKey: getEnv("INTERNAL_API_KEY", "default-api-key-for-development-only"),

		// Пагинация
		DefaultPaginationLimit: getEnvAsInt("DEFAULT_PAGINATION_LIMIT", 100),

		// Rate limiting
		RateLimitRequests: getEnvAsInt("RATE_LIMIT_REQUESTS", 100),
		RateLimitInterval: time.Duration(getEnvAsInt("RATE_LIMIT_INTERVAL_SECONDS", 60)) * time.Second,

		// gRPC сервер
		GRPCPort:             getEnv("GRPC_PORT", "50051"),
		GRPCMaxRecvMsgSize:   getEnvAsInt("GRPC_MAX_RECV_MSG_SIZE", 4*1024*1024), // 4 MB
		GRPCMaxSendMsgSize:   getEnvAsInt("GRPC_MAX_SEND_MSG_SIZE", 4*1024*1024), // 4 MB
		GRPCKeepAliveTime:    time.Duration(getEnvAsInt("GRPC_KEEP_ALIVE_TIME", 60)) * time.Second,
		GRPCKeepAliveTimeout: time.Duration(getEnvAsInt("GRPC_KEEP_ALIVE_TIMEOUT", 20)) * time.Second,
		EnableReflection:     getEnvAsBool("ENABLE_REFLECTION", true),
	}

	// Проверяем обязательные параметры
	if err := validateBaseConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// validateBaseConfig проверяет корректность базовой конфигурации
func validateBaseConfig(config *BaseConfig) error {
	// Проверяем обязательные параметры
	if config.ServiceName == "" {
		return fmt.Errorf("SERVICE_NAME must be set")
	}

	if config.Port == "" {
		return fmt.Errorf("PORT must be set")
	}

	return nil
}

// GetSecretFromEnvOrFile получает секрет либо из переменной окружения, либо из файла
func GetSecretFromEnvOrFile(envKey, fileEnvKey, defaultValue string) string {
	// Сначала проверяем, указан ли путь к файлу с секретом
	if fileEnvKey != "" {
		filePath := os.Getenv(fileEnvKey)
		if filePath != "" {
			// Если путь указан, читаем файл
			data, err := os.ReadFile(filePath)
			if err == nil && len(data) > 0 {
				return strings.TrimSpace(string(data))
			}
		}
	}

	// Если не удалось получить из файла, пробуем из переменной окружения
	value := os.Getenv(envKey)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnv получает значение переменной окружения или возвращает значение по умолчанию
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvAsInt получает значение переменной окружения как int или возвращает значение по умолчанию
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

// getEnvAsBool получает значение переменной окружения как bool или возвращает значение по умолчанию
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

// getEnvAsFloat получает значение переменной окружения как float64 или возвращает значение по умолчанию
func getEnvAsFloat(key string, defaultValue float64) float64 {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return defaultValue
	}

	return value
}

// getEnvAsSlice получает значение переменной окружения как слайс строк
func getEnvAsSlice(key string, defaultValue []string, separator string) []string {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	return strings.Split(valueStr, separator)
}

// getEnvAsDuration получает значение переменной окружения как time.Duration
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}
