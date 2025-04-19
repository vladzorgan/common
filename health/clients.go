package health

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/streadway/amqp"
	"gorm.io/gorm"
)

// DatabaseComponent представляет компонент проверки базы данных
type DatabaseComponent struct {
	name     string
	db       *gorm.DB
	critical bool
}

// NewDatabaseComponent создает новый компонент для проверки базы данных
func NewDatabaseComponent(name string, db *gorm.DB, critical bool) *DatabaseComponent {
	return &DatabaseComponent{
		name:     name,
		db:       db,
		critical: critical,
	}
}

// Name возвращает имя компонента
func (c *DatabaseComponent) Name() string {
	return c.name
}

// Check проверяет соединение с базой данных
func (c *DatabaseComponent) Check(ctx context.Context) (Status, error) {
	sqlDB, err := c.db.DB()
	if err != nil {
		return StatusDown, fmt.Errorf("cannot get database connection: %v", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return StatusDown, fmt.Errorf("database ping failed: %v", err)
	}

	return StatusUp, nil
}

// IsCritical возвращает true, если компонент критичен для работы сервиса
func (c *DatabaseComponent) IsCritical() bool {
	return c.critical
}

// SQLDatabaseComponent представляет компонент проверки SQL базы данных
type SQLDatabaseComponent struct {
	name     string
	db       *sql.DB
	critical bool
}

// NewSQLDatabaseComponent создает новый компонент для проверки SQL базы данных
func NewSQLDatabaseComponent(name string, db *sql.DB, critical bool) *SQLDatabaseComponent {
	return &SQLDatabaseComponent{
		name:     name,
		db:       db,
		critical: critical,
	}
}

// Name возвращает имя компонента
func (c *SQLDatabaseComponent) Name() string {
	return c.name
}

// Check проверяет соединение с базой данных
func (c *SQLDatabaseComponent) Check(ctx context.Context) (Status, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.db.PingContext(ctx); err != nil {
		return StatusDown, fmt.Errorf("database ping failed: %v", err)
	}

	return StatusUp, nil
}

// IsCritical возвращает true, если компонент критичен для работы сервиса
func (c *SQLDatabaseComponent) IsCritical() bool {
	return c.critical
}

// RedisComponent представляет компонент проверки Redis
type RedisComponent struct {
	name     string
	client   *redis.Client
	critical bool
}

// NewRedisComponent создает новый компонент для проверки Redis
func NewRedisComponent(name string, client *redis.Client, critical bool) *RedisComponent {
	return &RedisComponent{
		name:     name,
		client:   client,
		critical: critical,
	}
}

// Name возвращает имя компонента
func (c *RedisComponent) Name() string {
	return c.name
}

// Check проверяет соединение с Redis
func (c *RedisComponent) Check(ctx context.Context) (Status, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if _, err := c.client.Ping(ctx).Result(); err != nil {
		return StatusDown, fmt.Errorf("redis ping failed: %v", err)
	}

	return StatusUp, nil
}

// IsCritical возвращает true, если компонент критичен для работы сервиса
func (c *RedisComponent) IsCritical() bool {
	return c.critical
}

// RabbitMQComponent представляет компонент проверки RabbitMQ
type RabbitMQComponent struct {
	name     string
	url      string
	critical bool
}

// NewRabbitMQComponent создает новый компонент для проверки RabbitMQ
func NewRabbitMQComponent(name string, url string, critical bool) *RabbitMQComponent {
	return &RabbitMQComponent{
		name:     name,
		url:      url,
		critical: critical,
	}
}

// Name возвращает имя компонента
func (c *RabbitMQComponent) Name() string {
	return c.name
}

// Check проверяет соединение с RabbitMQ
func (c *RabbitMQComponent) Check(ctx context.Context) (Status, error) {
	// Устанавливаем соединение с RabbitMQ
	conn, err := amqp.Dial(c.url)
	if err != nil {
		return StatusDown, fmt.Errorf("rabbitmq connection failed: %v", err)
	}
	defer conn.Close()

	// Проверяем состояние соединения
	if conn.IsClosed() {
		return StatusDown, fmt.Errorf("rabbitmq connection is closed")
	}

	// Создаем канал
	ch, err := conn.Channel()
	if err != nil {
		return StatusDown, fmt.Errorf("rabbitmq channel creation failed: %v", err)
	}
	defer ch.Close()

	return StatusUp, nil
}

// IsCritical возвращает true, если компонент критичен для работы сервиса
func (c *RabbitMQComponent) IsCritical() bool {
	return c.critical
}

// ExternalServiceComponent представляет компонент проверки внешнего HTTP сервиса
type ExternalServiceComponent struct {
	name     string
	url      string
	timeout  time.Duration
	critical bool
}

// NewExternalServiceComponent создает новый компонент для проверки внешнего HTTP сервиса
func NewExternalServiceComponent(name string, url string, timeout time.Duration, critical bool) *ExternalServiceComponent {
	return &ExternalServiceComponent{
		name:     name,
		url:      url,
		timeout:  timeout,
		critical: critical,
	}
}

// Name возвращает имя компонента
func (c *ExternalServiceComponent) Name() string {
	return c.name
}

// Check проверяет доступность внешнего HTTP сервиса
func (c *ExternalServiceComponent) Check(ctx context.Context) (Status, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.url, nil)
	if err != nil {
		return StatusDown, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return StatusDown, fmt.Errorf("service request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return StatusUp, nil
	} else if resp.StatusCode >= 500 {
		return StatusDown, fmt.Errorf("service returned error status: %d", resp.StatusCode)
	} else {
		return StatusDegraded, fmt.Errorf("service returned unexpected status: %d", resp.StatusCode)
	}
}

// IsCritical возвращает true, если компонент критичен для работы сервиса
func (c *ExternalServiceComponent) IsCritical() bool {
	return c.critical
}
