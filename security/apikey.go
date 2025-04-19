// Package security предоставляет функции для обеспечения безопасности сервисов
package security

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vladzorgan/common/logging"
)

// APIKeyConfig содержит настройки для проверки API-ключа
type APIKeyConfig struct {
	// Заголовок, в котором передается API-ключ
	Header string
	// Ключ, который должен быть передан
	Key string
	// Список путей, которые не требуют проверки API-ключа
	ExcludedPaths []string
}

// DefaultAPIKeyConfig возвращает конфигурацию по умолчанию
func DefaultAPIKeyConfig() *APIKeyConfig {
	return &APIKeyConfig{
		Header: "X-API-Key",
		ExcludedPaths: []string{
			"/health",
			"/liveness",
			"/readiness",
			"/metrics",
		},
	}
}

// APIKeyMiddleware возвращает middleware для проверки API-ключа
func APIKeyMiddleware(config *APIKeyConfig, logger logging.Logger) gin.HandlerFunc {
	if config == nil {
		config = DefaultAPIKeyConfig()
	}

	if logger == nil {
		logger = logging.NewLogger()
	}

	return func(c *gin.Context) {
		// Проверяем, входит ли путь в список исключений
		path := c.Request.URL.Path
		method := c.Request.Method

		// OPTIONS запросы всегда пропускаем для CORS
		if method == http.MethodOptions {
			c.Next()
			return
		}

		// Проверяем, есть ли путь в исключениях
		for _, excludedPath := range config.ExcludedPaths {
			if path == excludedPath || path == excludedPath+"/" {
				c.Next()
				return
			}
		}

		// Получаем API-ключ из заголовка
		apiKey := c.GetHeader(config.Header)
		if apiKey == "" {
			logger.WithRequestID(c.GetString("RequestID")).
				WithField("path", path).
				WithField("method", method).
				Warn("API key is missing")

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "API key is required",
			})
			return
		}

		// Проверяем API-ключ
		if apiKey != config.Key {
			logger.WithRequestID(c.GetString("RequestID")).
				WithField("path", path).
				WithField("method", method).
				Warn("Invalid API key")

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Invalid API key",
			})
			return
		}

		c.Next()
	}
}

// InternalAPIKeyMiddleware возвращает middleware для проверки внутреннего API-ключа
func InternalAPIKeyMiddleware(apiKey string, excludedPaths []string, logger logging.Logger) gin.HandlerFunc {
	config := &APIKeyConfig{
		Header:        "X-Internal-API-Key",
		Key:           apiKey,
		ExcludedPaths: excludedPaths,
	}

	if config.ExcludedPaths == nil {
		config.ExcludedPaths = DefaultAPIKeyConfig().ExcludedPaths
	}

	return APIKeyMiddleware(config, logger)
}
