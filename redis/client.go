// Package redis предоставляет унифицированный интерфейс для работы с Redis
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/vladzorgan/common/logging"
)

// Client представляет клиент Redis
type Client struct {
	client *redis.Client
	logger logging.Logger
}

// ClientOptions содержит опции для создания клиента Redis
type ClientOptions struct {
	// Максимальное количество соединений
	PoolSize int
	// Минимальное количество простаивающих соединений
	MinIdleConns int
	// Время ожидания при получении соединения из пула
	PoolTimeout time.Duration
	// Время ожидания операций с Redis
	ReadTimeout time.Duration
	// Время ожидания записи в Redis
	WriteTimeout time.Duration
}

// DefaultClientOptions возвращает опции по умолчанию
func DefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		PoolSize:     10,
		MinIdleConns: 5,
		PoolTimeout:  time.Second * 4,
		ReadTimeout:  time.Second * 3,
		WriteTimeout: time.Second * 3,
	}
}

// NewClient создает новый клиент Redis
func NewClient(addr string, password string, db int, logger logging.Logger, options *ClientOptions) (*Client, error) {
	if logger == nil {
		logger = logging.NewLogger()
	}

	if options == nil {
		options = DefaultClientOptions()
	}

	// Создаем клиент Redis
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     options.PoolSize,
		MinIdleConns: options.MinIdleConns,
		PoolTimeout:  options.PoolTimeout,
		ReadTimeout:  options.ReadTimeout,
		WriteTimeout: options.WriteTimeout,
	})

	// Проверяем соединение
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	logger.Info("Successfully connected to Redis")

	return &Client{
		client: client,
		logger: logger,
	}, nil
}

// Close закрывает соединение с Redis
func (c *Client) Close() error {
	if err := c.client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis connection: %v", err)
	}

	c.logger.Info("Redis connection closed")
	return nil
}

// Client возвращает оригинальный клиент Redis
func (c *Client) Client() *redis.Client {
	return c.client
}

// Ping проверяет соединение с Redis
func (c *Client) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis ping failed: %v", err)
	}

	return nil
}

// Get получает значение по ключу
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	result, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Ключ не найден
	} else if err != nil {
		return "", fmt.Errorf("failed to get value from Redis: %v", err)
	}

	return result, nil
}

// Set устанавливает значение по ключу
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if err := c.client.Set(ctx, key, value, expiration).Err(); err != nil {
		return fmt.Errorf("failed to set value in Redis: %v", err)
	}

	return nil
}

// Del удаляет ключ
func (c *Client) Del(ctx context.Context, keys ...string) error {
	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete keys from Redis: %v", err)
	}

	return nil
}

// Exists проверяет существование ключа
func (c *Client) Exists(ctx context.Context, keys ...string) (bool, error) {
	result, err := c.client.Exists(ctx, keys...).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence in Redis: %v", err)
	}

	return result > 0, nil
}

// Expire устанавливает время жизни ключа
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if err := c.client.Expire(ctx, key, expiration).Err(); err != nil {
		return fmt.Errorf("failed to set expiration in Redis: %v", err)
	}

	return nil
}

// TTL возвращает оставшееся время жизни ключа
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	result, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL from Redis: %v", err)
	}

	return result, nil
}

// Incr увеличивает значение ключа на 1
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	result, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment value in Redis: %v", err)
	}

	return result, nil
}

// SetJSON устанавливает JSON значение по ключу
func (c *Client) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	// Маршалим значение в JSON
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Устанавливаем значение в Redis
	if err := c.client.Set(ctx, key, data, expiration).Err(); err != nil {
		return fmt.Errorf("failed to set JSON in Redis: %v", err)
	}

	return nil
}

// GetJSON получает JSON значение по ключу и десериализует его в указанный тип
func (c *Client) GetJSON(ctx context.Context, key string, value interface{}) error {
	// Получаем значение из Redis
	data, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil // Ключ не найден
	} else if err != nil {
		return fmt.Errorf("failed to get JSON from Redis: %v", err)
	}

	// Анмаршалим значение из JSON
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	return nil
}

// HSet устанавливает поле хеша
func (c *Client) HSet(ctx context.Context, key, field string, value interface{}) error {
	if err := c.client.HSet(ctx, key, field, value).Err(); err != nil {
		return fmt.Errorf("failed to set hash field in Redis: %v", err)
	}

	return nil
}

// HGet получает поле хеша
func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	result, err := c.client.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return "", nil // Поле не найдено
	} else if err != nil {
		return "", fmt.Errorf("failed to get hash field from Redis: %v", err)
	}

	return result, nil
}

// HGetAll получает все поля хеша
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	result, err := c.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get all hash fields from Redis: %v", err)
	}

	return result, nil
}

// HDel удаляет поля хеша
func (c *Client) HDel(ctx context.Context, key string, fields ...string) error {
	if err := c.client.HDel(ctx, key, fields...).Err(); err != nil {
		return fmt.Errorf("failed to delete hash fields from Redis: %v", err)
	}

	return nil
}

// Publish публикует сообщение в канал
func (c *Client) Publish(ctx context.Context, channel string, message interface{}) error {
	if err := c.client.Publish(ctx, channel, message).Err(); err != nil {
		return fmt.Errorf("failed to publish message to Redis: %v", err)
	}

	return nil
}

// Subscribe подписывается на канал
func (c *Client) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return c.client.Subscribe(ctx, channel)
}

// LPush добавляет элементы в начало списка
func (c *Client) LPush(ctx context.Context, key string, values ...interface{}) error {
	if err := c.client.LPush(ctx, key, values...).Err(); err != nil {
		return fmt.Errorf("failed to push to list in Redis: %v", err)
	}

	return nil
}

// RPush добавляет элементы в конец списка
func (c *Client) RPush(ctx context.Context, key string, values ...interface{}) error {
	if err := c.client.RPush(ctx, key, values...).Err(); err != nil {
		return fmt.Errorf("failed to push to list in Redis: %v", err)
	}

	return nil
}

// LPop удаляет и возвращает первый элемент списка
func (c *Client) LPop(ctx context.Context, key string) (string, error) {
	result, err := c.client.LPop(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Список пуст
	} else if err != nil {
		return "", fmt.Errorf("failed to pop from list in Redis: %v", err)
	}

	return result, nil
}

// RPop удаляет и возвращает последний элемент списка
func (c *Client) RPop(ctx context.Context, key string) (string, error) {
	result, err := c.client.RPop(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Список пуст
	} else if err != nil {
		return "", fmt.Errorf("failed to pop from list in Redis: %v", err)
	}

	return result, nil
}

// LRange возвращает диапазон элементов списка
func (c *Client) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	result, err := c.client.LRange(ctx, key, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get range from list in Redis: %v", err)
	}

	return result, nil
}
