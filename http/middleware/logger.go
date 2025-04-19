// Package middleware предоставляет набор middleware для HTTP сервера
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vladzorgan/common/logging"
)

// RequestIDHeader определяет заголовок для идентификатора запроса
const RequestIDHeader = "X-Request-ID"

// Logger возвращает middleware для логирования запросов
func Logger(logger logging.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Время начала запроса
		startTime := time.Now()

		// Получаем идентификатор запроса
		requestID := c.GetString("RequestID")
		if requestID == "" {
			requestID = c.GetHeader(RequestIDHeader)
			if requestID == "" {
				requestID = uuid.New().String()
			}
			c.Set("RequestID", requestID)
		}

		// Создаем логгер с данными запроса
		reqLogger := logger.WithRequestID(requestID).
			WithField("method", c.Request.Method).
			WithField("path", c.Request.URL.Path).
			WithField("client_ip", c.ClientIP())

		reqLogger.Info("Request started")

		// Обрабатываем запрос
		c.Next()

		// Вычисляем время выполнения запроса
		latency := time.Since(startTime)

		// Формируем сообщение лога в зависимости от статуса
		statusCode := c.Writer.Status()
		fields := map[string]interface{}{
			"status":     statusCode,
			"latency_ms": latency.Milliseconds(),
			"body_size":  c.Writer.Size(),
			"user_agent": c.Request.UserAgent(),
			"referer":    c.Request.Referer(),
		}

		// Добавляем ошибки, если есть
		if len(c.Errors) > 0 {
			fields["errors"] = c.Errors.String()
		}

		reqLogger = reqLogger.WithFields(fields)

		// Логируем информацию о запросе в зависимости от статуса
		if statusCode >= 500 {
			reqLogger.Error("Server error")
		} else if statusCode >= 400 {
			reqLogger.Warn("Client error")
		} else {
			reqLogger.Info("Request completed")
		}
	}
}

// RequestID возвращает middleware для генерации уникального идентификатора запроса
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Проверяем, передан ли идентификатор в заголовке
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			// Если нет, генерируем новый
			requestID = uuid.New().String()
		}

		// Устанавливаем идентификатор в контекст и заголовок ответа
		c.Set("RequestID", requestID)
		c.Writer.Header().Set(RequestIDHeader, requestID)

		c.Next()
	}
}

// Recovery возвращает middleware для восстановления после паники
func Recovery(logger logging.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Получаем идентификатор запроса
				requestID := c.GetString("RequestID")
				if requestID == "" {
					requestID = uuid.New().String()
					c.Set("RequestID", requestID)
				}

				// Логируем ошибку
				logger.WithRequestID(requestID).
					WithField("method", c.Request.Method).
					WithField("path", c.Request.URL.Path).
					WithField("client_ip", c.ClientIP()).
					Error("Panic recovered: %v", err)

				// Возвращаем 500 Internal Server Error
				c.AbortWithStatus(500)
			}
		}()

		c.Next()
	}
}
