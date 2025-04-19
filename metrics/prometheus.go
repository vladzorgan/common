// Package metrics предоставляет унифицированный интерфейс для сбора и экспорта метрик
package metrics

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// RequestsTotal счетчик общего числа запросов
	RequestsTotal *prometheus.CounterVec

	// RequestDuration гистограмма времени обработки запросов
	RequestDuration *prometheus.HistogramVec

	// ResponseSize гистограмма размера ответов
	ResponseSize *prometheus.HistogramVec

	// ActiveRequests счетчик активных запросов
	ActiveRequests prometheus.Gauge

	// ServerUptime счетчик времени работы сервера
	ServerUptime prometheus.Counter

	// CustomMetrics карта пользовательских метрик
	CustomMetrics map[string]interface{}
)

// InitMetrics инициализирует метрики Prometheus
func InitMetrics(servicePrefix string) {
	// Инициализируем карту пользовательских метрик
	CustomMetrics = make(map[string]interface{})

	// Счетчик общего числа запросов
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: servicePrefix + "_requests_total",
			Help: "Общее количество запросов к сервису",
		},
		[]string{"method", "path", "status"},
	)

	// Гистограмма времени обработки запросов
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    servicePrefix + "_request_duration_ms",
			Help:    "Продолжительность запроса в миллисекундах",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15), // От 1мс до ~16с
		},
		[]string{"method", "path", "status"},
	)

	// Гистограмма размера ответов
	ResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    servicePrefix + "_response_size_bytes",
			Help:    "Размер ответа в байтах",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8), // От 100Б до ~100МБ
		},
		[]string{"method", "path"},
	)

	// Счетчик активных запросов
	ActiveRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: servicePrefix + "_active_requests",
			Help: "Количество активных запросов",
		},
	)

	// Счетчик времени работы сервера
	ServerUptime = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: servicePrefix + "_uptime_seconds",
			Help: "Время работы сервера в секундах",
		},
	)
}

// RecordRequest записывает метрики о запросе
func RecordRequest(method, path string, status int, durationMs float64, sizeBytes int64) {
	statusStr := string(rune(status))
	RequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	RequestDuration.WithLabelValues(method, path, statusStr).Observe(durationMs)
	ResponseSize.WithLabelValues(method, path).Observe(float64(sizeBytes))
}

// IncrementActiveRequests увеличивает счетчик активных запросов
func IncrementActiveRequests() {
	ActiveRequests.Inc()
}

// DecrementActiveRequests уменьшает счетчик активных запросов
func DecrementActiveRequests() {
	ActiveRequests.Dec()
}

// UpdateUptime обновляет счетчик времени работы сервера
func UpdateUptime(startTime time.Time) {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			ServerUptime.Add(1)
		}
	}()
}

// RegisterCounter регистрирует и возвращает новый счетчик
func RegisterCounter(servicePrefix, name, help string, labelNames ...string) *prometheus.CounterVec {
	counter := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: servicePrefix + "_" + name,
			Help: help,
		},
		labelNames,
	)
	CustomMetrics[name] = counter
	return counter
}

// RegisterGauge регистрирует и возвращает новый gauge
func RegisterGauge(servicePrefix, name, help string, labelNames ...string) *prometheus.GaugeVec {
	gauge := promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: servicePrefix + "_" + name,
			Help: help,
		},
		labelNames,
	)
	CustomMetrics[name] = gauge
	return gauge
}

// RegisterHistogram регистрирует и возвращает новую гистограмму
func RegisterHistogram(servicePrefix, name, help string, buckets []float64, labelNames ...string) *prometheus.HistogramVec {
	histogram := promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    servicePrefix + "_" + name,
			Help:    help,
			Buckets: buckets,
		},
		labelNames,
	)
	CustomMetrics[name] = histogram
	return histogram
}

// RegisterSummary регистрирует и возвращает новое summary
func RegisterSummary(servicePrefix, name, help string, objectives map[float64]float64, labelNames ...string) *prometheus.SummaryVec {
	summary := promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       servicePrefix + "_" + name,
			Help:       help,
			Objectives: objectives,
		},
		labelNames,
	)
	CustomMetrics[name] = summary
	return summary
}

// MetricsMiddleware возвращает middleware для сбора метрик
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Увеличиваем счетчик активных запросов
		IncrementActiveRequests()
		defer DecrementActiveRequests()

		// Запоминаем время начала запроса
		startTime := time.Now()

		// Обрабатываем запрос
		c.Next()

		// Вычисляем продолжительность запроса
		duration := time.Since(startTime)

		// Обновляем метрики
		RecordRequest(
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			duration.Seconds()*1000, // миллисекунды
			int64(c.Writer.Size()),
		)
	}
}

// PrometheusHandler возвращает обработчик для метрик Prometheus
func PrometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()

	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
