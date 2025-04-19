// Package health предоставляет интерфейсы и реализации для проверки здоровья сервиса
package health

import (
	"context"
	"sync"
	"time"
)

// Status представляет статус компонента или сервиса
type Status string

const (
	// StatusUp компонент работает нормально
	StatusUp Status = "up"
	// StatusDown компонент не работает
	StatusDown Status = "down"
	// StatusDegraded компонент работает, но с проблемами
	StatusDegraded Status = "degraded"
)

// Component представляет компонент, который можно проверить
type Component interface {
	// Name возвращает имя компонента
	Name() string
	// Check проверяет состояние компонента
	Check(ctx context.Context) (Status, error)
	// IsCritical возвращает true, если компонент критичен для работы сервиса
	IsCritical() bool
}

// CheckResult представляет результат проверки компонента
type CheckResult struct {
	Component string    `json:"component"`
	Status    Status    `json:"status"`
	Error     *string   `json:"error,omitempty"`
	Time      time.Time `json:"time"`
	Duration  int64     `json:"duration_ms"`
}

// HealthCheck представляет результат проверки здоровья всего сервиса
type HealthCheck struct {
	Status        Status                 `json:"status"`
	ServiceName   string                 `json:"service_name"`
	ServicePrefix string                 `json:"service_prefix"`
	Version       string                 `json:"version"`
	Uptime        float64                `json:"uptime"`
	Timestamp     float64                `json:"timestamp"`
	CheckDuration float64                `json:"check_duration_ms"`
	Components    map[string]interface{} `json:"components"`
}

// Checker представляет сервис для проверки здоровья
type Checker struct {
	startTime     time.Time
	serviceName   string
	servicePrefix string
	version       string
	components    []Component
	mutex         sync.RWMutex
}

// NewChecker создает новый сервис проверки здоровья
func NewChecker(serviceName, servicePrefix, version string) *Checker {
	return &Checker{
		startTime:     time.Now(),
		serviceName:   serviceName,
		servicePrefix: servicePrefix,
		version:       version,
		components:    make([]Component, 0),
	}
}

// RegisterComponent регистрирует компонент для проверки
func (c *Checker) RegisterComponent(component Component) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.components = append(c.components, component)
}

// Check проверяет здоровье всех зарегистрированных компонентов
func (c *Checker) Check(ctx context.Context) (*HealthCheck, error) {
	startTime := time.Now()

	// Копируем список компонентов для потокобезопасности
	c.mutex.RLock()
	components := make([]Component, len(c.components))
	copy(components, c.components)
	c.mutex.RUnlock()

	results := make(map[string]interface{})
	overallStatus := StatusUp

	// Проверяем каждый компонент
	for _, component := range components {
		checkStartTime := time.Now()
		status, err := component.Check(ctx)
		duration := time.Since(checkStartTime).Milliseconds()

		var errStr *string
		if err != nil {
			errMsg := err.Error()
			errStr = &errMsg
		}

		results[component.Name()] = CheckResult{
			Component: component.Name(),
			Status:    status,
			Error:     errStr,
			Time:      checkStartTime,
			Duration:  duration,
		}

		// Определение общего статуса
		if status == StatusDown && component.IsCritical() {
			overallStatus = StatusDown
		} else if status == StatusDegraded && overallStatus != StatusDown {
			overallStatus = StatusDegraded
		}
	}

	return &HealthCheck{
		Status:        overallStatus,
		ServiceName:   c.serviceName,
		ServicePrefix: c.servicePrefix,
		Version:       c.version,
		Uptime:        time.Since(c.startTime).Seconds(),
		Timestamp:     float64(time.Now().Unix()),
		CheckDuration: float64(time.Since(startTime).Milliseconds()),
		Components:    results,
	}, nil
}
