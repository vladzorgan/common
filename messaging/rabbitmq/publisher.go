// Package rabbitmq предоставляет унифицированный интерфейс для работы с RabbitMQ
package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/streadway/amqp"
	"github.com/vladzorgan/common/logging"
)

// PublishConfig содержит настройки для публикации сообщений
type PublishConfig struct {
	Mandatory bool
	Immediate bool
	Headers   map[string]interface{}
	Priority  uint8
}

// EventEnvelope представляет конверт для события
type EventEnvelope struct {
	EventType   string      `json:"event_type"`
	OccurredAt  time.Time   `json:"occurred_at"`
	ServiceName string      `json:"service_name"`
	Payload     interface{} `json:"payload"`
}

// Publisher представляет сервис для публикации событий в RabbitMQ
type Publisher struct {
	connection   *amqp.Connection
	channel      *amqp.Channel
	exchangeName string
	serviceName  string
	logger       logging.Logger
	mutex        sync.RWMutex
	connected    bool
	reconnecting bool
}

// NewPublisher создает новый экземпляр Publisher
func NewPublisher(rabbitmqURL, exchangeName, serviceName string, logger logging.Logger) (*Publisher, error) {
	if logger == nil {
		logger = logging.NewLogger()
	}

	if rabbitmqURL == "" {
		logger.Warn("RABBITMQ_URL not set, events will not be published")
		return &Publisher{
			exchangeName: exchangeName,
			serviceName:  serviceName,
			logger:       logger,
		}, nil
	}

	publisher := &Publisher{
		exchangeName: exchangeName,
		serviceName:  serviceName,
		logger:       logger,
	}

	if err := publisher.connect(rabbitmqURL); err != nil {
		logger.Error("Failed to connect to RabbitMQ: %v", err)
		go publisher.reconnect(rabbitmqURL)
		return publisher, nil
	}

	return publisher, nil
}

// connect устанавливает соединение с RabbitMQ
func (p *Publisher) connect(rabbitmqURL string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.connected {
		return nil
	}

	// Подключаемся к RabbitMQ
	connection, err := amqp.Dial(rabbitmqURL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	// Создаем канал
	channel, err := connection.Channel()
	if err != nil {
		connection.Close()
		return fmt.Errorf("failed to create channel: %v", err)
	}

	// Объявляем обменник
	err = channel.ExchangeDeclare(
		p.exchangeName, // имя обменника
		"topic",        // тип обменника
		true,           // долговечный (durable)
		false,          // автоудаляемый (auto-delete)
		false,          // внутренний (internal)
		false,          // не ждать подтверждения (no-wait)
		nil,            // аргументы
	)
	if err != nil {
		channel.Close()
		connection.Close()
		return fmt.Errorf("failed to declare exchange: %v", err)
	}

	// Устанавливаем обработчик закрытия соединения
	closeChan := make(chan *amqp.Error)
	connection.NotifyClose(closeChan)

	// Запускаем горутину для мониторинга состояния соединения
	go func() {
		// Ждем закрытия соединения
		err := <-closeChan
		p.logger.Warn("RabbitMQ connection closed: %v", err)
		p.mutex.Lock()
		p.connected = false
		p.mutex.Unlock()

		// Переподключаемся
		go p.reconnect(rabbitmqURL)
	}()

	p.connection = connection
	p.channel = channel
	p.connected = true

	p.logger.Info("Successfully connected to RabbitMQ")
	return nil
}

// reconnect пытается переподключиться к RabbitMQ
func (p *Publisher) reconnect(rabbitmqURL string) {
	p.mutex.Lock()
	if p.reconnecting {
		p.mutex.Unlock()
		return
	}
	p.reconnecting = true
	p.mutex.Unlock()

	defer func() {
		p.mutex.Lock()
		p.reconnecting = false
		p.mutex.Unlock()
	}()

	// Бесконечные попытки подключения
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		p.logger.Info("Trying to reconnect to RabbitMQ in %v...", backoff)
		time.Sleep(backoff)

		// Пытаемся подключиться
		if err := p.connect(rabbitmqURL); err != nil {
			p.logger.Error("Failed to reconnect to RabbitMQ: %v", err)

			// Увеличиваем время ожидания (экспоненциальный backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		p.logger.Info("Successfully reconnected to RabbitMQ")
		return
	}
}

// Close закрывает соединение с RabbitMQ
func (p *Publisher) Close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.channel != nil {
		p.channel.Close()
	}
	if p.connection != nil {
		p.connection.Close()
	}

	p.connected = false
}

// PublishEvent публикует событие в RabbitMQ
func (p *Publisher) PublishEvent(ctx context.Context, routingKey string, payload interface{}) error {
	return p.PublishEventWithConfig(ctx, routingKey, payload, nil)
}

// PublishEventWithConfig публикует событие в RabbitMQ с дополнительными настройками
func (p *Publisher) PublishEventWithConfig(ctx context.Context, routingKey string, payload interface{}, config *PublishConfig) error {
	// Если соединение не установлено, просто логируем событие
	p.mutex.RLock()
	if p.channel == nil {
		p.mutex.RUnlock()
		p.logger.Debug("Event %s not published (RabbitMQ not connected): %+v", routingKey, payload)
		return nil
	}
	p.mutex.RUnlock()

	// Создаем конверт для события
	envelope := EventEnvelope{
		EventType:   routingKey,
		OccurredAt:  time.Now(),
		ServiceName: p.serviceName,
		Payload:     payload,
	}

	// Сериализуем конверт в JSON
	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %v", err)
	}

	// Создаем сообщение
	msg := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		ContentType:  "application/json",
		Body:         body,
		MessageId:    fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	// Применяем дополнительные настройки, если указаны
	if config != nil {
		if config.Headers != nil {
			msg.Headers = config.Headers
		}
		if config.Priority > 0 {
			msg.Priority = config.Priority
		}
	}

	// Публикуем сообщение
	p.mutex.RLock()
	err = p.channel.Publish(
		p.exchangeName,                    // обменник
		routingKey,                        // ключ маршрутизации
		config != nil && config.Mandatory, // обязательный (mandatory)
		config != nil && config.Immediate, // мгновенный (immediate)
		msg,
	)
	p.mutex.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to publish message: %v", err)
	}

	p.logger.Debug("Published event %s", routingKey)
	return nil
}
