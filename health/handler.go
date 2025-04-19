package health

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HTTPHandler представляет обработчик HTTP для проверки здоровья
type HTTPHandler struct {
	checker *Checker
}

// NewHTTPHandler создает новый HTTP обработчик для проверки здоровья
func NewHTTPHandler(checker *Checker) *HTTPHandler {
	return &HTTPHandler{
		checker: checker,
	}
}

// HealthCheck обрабатывает запрос проверки здоровья сервиса
// @Summary Проверка здоровья сервиса
// @Description Проверяет здоровье сервиса и его зависимостей
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} HealthCheck
// @Failure 503 {object} HealthCheck
// @Router /health [get]
func (h *HTTPHandler) HealthCheck(c *gin.Context) {
	// Проверяем здоровье сервиса
	health, err := h.checker.Check(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Устанавливаем HTTP статус в зависимости от состояния сервиса
	httpStatus := http.StatusOK
	if health.Status == StatusDown {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, health)
}

// LivenessCheck обрабатывает запрос проверки готовности сервиса
// @Summary Проверка готовности сервиса
// @Description Проверяет готовность сервиса к работе (liveness)
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /liveness [get]
func (h *HTTPHandler) LivenessCheck(c *gin.Context) {
	// Для liveness проверки мы просто возвращаем 200 OK если приложение запущено
	c.JSON(http.StatusOK, gin.H{
		"status":    "up",
		"timestamp": time.Now().Unix(),
	})
}

// ReadinessCheck обрабатывает запрос проверки готовности сервиса к обработке запросов
// @Summary Проверка готовности сервиса к обработке запросов
// @Description Проверяет, готов ли сервис к обработке запросов (readiness)
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Router /readiness [get]
func (h *HTTPHandler) ReadinessCheck(c *gin.Context) {
	// Проверяем здоровье сервиса
	health, err := h.checker.Check(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":    "down",
			"timestamp": time.Now().Unix(),
			"error":     err.Error(),
		})
		return
	}

	// Для readiness проверки мы проверяем все компоненты
	httpStatus := http.StatusOK
	if health.Status == StatusDown {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status":    string(health.Status),
		"timestamp": time.Now().Unix(),
	})
}

// RegisterHandlers регистрирует обработчики проверки здоровья в Gin роутере
func (h *HTTPHandler) RegisterHandlers(router *gin.Engine) {
	router.GET("/health", h.HealthCheck)
	router.GET("/liveness", h.LivenessCheck)
	router.GET("/readiness", h.ReadinessCheck)
}

// RegisterHandlersGroup регистрирует обработчики проверки здоровья в Gin группе
func (h *HTTPHandler) RegisterHandlersGroup(group *gin.RouterGroup) {
	group.GET("/health", h.HealthCheck)
	group.GET("/liveness", h.LivenessCheck)
	group.GET("/readiness", h.ReadinessCheck)
}
