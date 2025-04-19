package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rem-consultant/common/logging"
	"github.com/streadway/amqp"
)

// HandlerFunc представляет функцию-обработчик сообщений
type HandlerFunc func(ctx context.Context, delivery amqp.Delivery, message []byte) error

// Consumer представляет потребителя сообщений из RabbitMQ
type Consumer struct {
	connection   *amqp.Connection
	channel      *amqp.Channel
	exchangeName string
	queueName    string
	serviceName  string
	logger       logging.Logger
	handlers     map[string]HandlerFunc
	mutex        sync.RWMutex
	connected    bool
	reconnecting bool
	stopChan     chan struct{}
	stopped      bool
}

// ConsumerOptions содержит опции для создания потребителя
type ConsumerOptions struct {
	QueueDurable    bool
	QueueAutoDelete bool
	QueueExclusive  bool
	QueueNoWait     bool
	QueueArgs       map[string]interface{}
	PrefetchCount   int
	PrefetchSize    int
	PrefetchGlobal  bool
}

// DefaultConsumerOptions возвращает опции по умолчанию
func DefaultConsumerOptions() *ConsumerOptions {
	return &ConsumerOptions{
		QueueDurable:    true,
		QueueAutoDelete: false,
		QueueExclusive:  false,
		QueueNoWait:     false,
		QueueArgs:       nil,
		PrefetchCount:   1,
		PrefetchSize:    0,
		PrefetchGlobal:  false,
	}
}

// NewConsumer создает нового потребителя сообщений
func NewConsumer(
	rabbitmqURL string,
	exchangeName string,
	queueName string,
	serviceName string,
	logger logging.Logger,
	options *ConsumerOptions,
) (*Consumer, error) {
	if logger == nil {
		logger = logging.NewLogger()
	}

	if options == nil {
		options = DefaultConsumerOptions()
	}

	consumer := &Consumer{
		exchangeName: exchangeName,
		queueName:    queueName,
		serviceName:  serviceName,
		logger:       logger,
		handlers:     make(map[string]HandlerFunc),
		stopChan:     make(chan struct{}),
	}

	if rabbitmqURL == "" {
		logger.Warn("RABBITMQ_URL not set, events will not be consumed")
		return consumer, nil
	}

	if err := consumer.connect(rabbitmqURL, options); err != nil {
		logger.Error("Failed to connect to RabbitMQ: %v", err)
		go consumer.reconnect(rabbitmqURL, options)
		return consumer, nil
	}

	return consumer, nil
}

// connect устанавливает соединение с RabbitMQ
func (c *Consumer) connect(rabbitmqURL string, options *ConsumerOptions) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.connected {
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

	// Настраиваем prefetch
	if err := channel.Qos(
		options.PrefetchCount,
		options.PrefetchSize,
		options.PrefetchGlobal,
	); err != nil {
		channel.Close()
		connection.Close()
		return fmt.Errorf("failed to set QoS: %v", err)
	}

	// Объявляем обменник
	err = channel.ExchangeDeclare(
		c.exchangeName, // имя обменника
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

	// Объявляем очередь
	_, err = channel.QueueDeclare(
		c.queueName,             // имя очереди
		options.QueueDurable,    // долговечная (durable)
		options.QueueAutoDelete, // автоудаляемая (auto-delete)
		options.QueueExclusive,  // эксклюзивная (exclusive)
		options.QueueNoWait,     // не ждать подтверждения (no-wait)
		options.QueueArgs,       // аргументы
	)
	if err != nil {
		channel.Close()
		connection.Close()
		return fmt.Errorf("failed to declare queue: %v", err)
	}

	// Устанавливаем обработчик закрытия соединения
	closeChan := make(chan *amqp.Error)
	connection.NotifyClose(closeChan)

	// Запускаем горутину для мониторинга состояния соединения
	go func() {
		select {
		case err := <-closeChan:
			c.logger.Warn("RabbitMQ connection closed: %v", err)
			c.mutex.Lock()
			c.connected = false
			c.mutex.Unlock()

			// Переподключаемся
			go c.reconnect(rabbitmqURL, options)
		case <-c.stopChan:
			c.logger.Info("Consumer stopped, closing connection")
			c.mutex.Lock()
			if c.channel != nil {
				c.channel.Close()
			}
			if c.connection != nil {
				c.connection.Close()
			}
			c.connected = false
			c.mutex.Unlock()
		}
	}()

	c.connection = connection
	c.channel = channel
	c.connected = true

	c.logger.Info("Successfully connected to RabbitMQ")
	return nil
}

// reconnect пытается переподключиться к RabbitMQ
func (c *Consumer) reconnect(rabbitmqURL string, options *ConsumerOptions) {
	c.mutex.Lock()
	if c.reconnecting || c.stopped {
		c.mutex.Unlock()
		return
	}
	c.reconnecting = true
	c.mutex.Unlock()

	defer func() {
		c.mutex.Lock()
		c.reconnecting = false
		c.mutex.Unlock()
	}()

	// Бесконечные попытки подключения
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		c.mutex.RLock()
		stopped := c.stopped
		c.mutex.RUnlock()

		if stopped {
			c.logger.Info("Consumer stopped, aborting reconnection")
			return
		}

		c.logger.Info("Trying to reconnect to RabbitMQ in %v...", backoff)
		time.Sleep(backoff)

		// Пытаемся подключиться
		if err := c.connect(rabbitmqURL, options); err != nil {
			c.logger.Error("Failed to reconnect to RabbitMQ: %v", err)

			// Увеличиваем время ожидания (экспоненциальный backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Повторно подписываемся на все маршруты
		if err := c.resubscribe(); err != nil {
			c.logger.Error("Failed to resubscribe to routes: %v", err)
			c.mutex.Lock()
			c.connected = false
			c.mutex.Unlock()
			continue
		}

		c.logger.Info("Successfully reconnected to RabbitMQ")
		return
	}
}

// resubscribe повторно подписывается на все маршруты
func (c *Consumer) resubscribe() error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if !c.connected || c.channel == nil {
		return fmt.Errorf("not connected to RabbitMQ")
	}

	// Копируем маршруты
	routes := make([]string, 0, len(c.handlers))
	for route := range c.handlers {
		routes = append(routes, route)
	}

	// Подписываемся на все маршруты
	for _, route := range routes {
		if err := c.channel.QueueBind(
			c.queueName,    // имя очереди
			route,          // ключ маршрутизации
			c.exchangeName, // имя обменника
			false,          // не ждать подтверждения (no-wait)
			nil,            // аргументы
		); err != nil {
			return fmt.Errorf("failed to bind queue to exchange: %v", err)
		}
	}

	// Начинаем потреблять сообщения
	deliveries, err := c.channel.Consume(
		c.queueName, // имя очереди
		"",          // потребитель
		false,       // автоматическое подтверждение
		false,       // эксклюзивный (exclusive)
		false,       // локальный (no-local)
		false,       // не ждать подтверждения (no-wait)
		nil,         // аргументы
	)
	if err != nil {
		return fmt.Errorf("failed to consume from queue: %v", err)
	}

	// Запускаем обработчик сообщений
	go c.handleDeliveries(deliveries)

	return nil
}

// Subscribe подписывается на указанный маршрут
func (c *Consumer) Subscribe(routingKey string, handler HandlerFunc) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Сохраняем обработчик
	c.handlers[routingKey] = handler

	// Если не подключены, просто сохраняем обработчик
	if !c.connected || c.channel == nil {
		return nil
	}

	// Связываем очередь с обменником
	if err := c.channel.QueueBind(
		c.queueName,    // имя очереди
		routingKey,     // ключ маршрутизации
		c.exchangeName, // имя обменника
		false,          // не ждать подтверждения (no-wait)
		nil,            // аргументы
	); err != nil {
		return fmt.Errorf("failed to bind queue to exchange: %v", err)
	}

	// Если это первая подписка, начинаем потреблять сообщения
	if len(c.handlers) == 1 {
		deliveries, err := c.channel.Consume(
			c.queueName, // имя очереди
			"",          // потребитель
			false,       // автоматическое подтверждение
			false,       // эксклюзивный (exclusive)
			false,       // локальный (no-local)
			false,       // не ждать подтверждения (no-wait)
			nil,         // аргументы
		)
		if err != nil {
			return fmt.Errorf("failed to consume from queue: %v", err)
		}

		// Запускаем обработчик сообщений
		go c.handleDeliveries(deliveries)
	}

	return nil
}

// handleDeliveries обрабатывает поступающие сообщения
func (c *Consumer) handleDeliveries(deliveries <-chan amqp.Delivery) {
	for delivery := range deliveries {
		// Создаем контекст с timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		// Получаем обработчик для данного маршрута
		c.mutex.RLock()
		handler, ok := c.handlers[delivery.RoutingKey]
		c.mutex.RUnlock()

		if !ok {
			c.logger.Warn("No handler for routing key %s", delivery.RoutingKey)
			delivery.Nack(false, false) // Не переотправляем
			cancel()
			continue
		}

		// Обрабатываем сообщение
		c.logger.Debug("Processing message with routing key: %s", delivery.RoutingKey)

		// Распаковываем конверт события
		var envelope EventEnvelope
		err := json.Unmarshal(delivery.Body, &envelope)
		if err != nil {
			c.logger.Error("Failed to unmarshal message: %v", err)
			delivery.Nack(false, false) // Не переотправляем при ошибке формата
			cancel()
			continue
		}

		// Преобразуем payload в JSON
		payload, err := json.Marshal(envelope.Payload)
		if err != nil {
			c.logger.Error("Failed to marshal payload: %v", err)
			delivery.Nack(false, false)
			cancel()
			continue
		}

		// Обогащаем контекст данными события
		ctx = context.WithValue(ctx, "event_type", envelope.EventType)
		ctx = context.WithValue(ctx, "occurred_at", envelope.OccurredAt)
		ctx = context.WithValue(ctx, "service_name", envelope.ServiceName)
		ctx = logging.ContextWithRequestID(ctx, delivery.MessageId)

		// Вызываем обработчик
		err = handler(ctx, delivery, payload)
		if err != nil {
			c.logger.Error("Failed to process message: %v", err)
			// При ошибке обработки ставим сообщение обратно в очередь
			// Можно также реализовать DLX (Dead Letter Exchange) для обработки ошибок
			delivery.Nack(false, true)
		} else {
			delivery.Ack(false)
		}

		cancel()
	}

	c.logger.Warn("Delivery channel closed")
}

// Close закрывает соединение с RabbitMQ
func (c *Consumer) Close() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.stopped {
		return
	}

	c.stopped = true
	close(c.stopChan)

	if c.channel != nil {
		c.channel.Close()
		c.channel = nil
	}
	if c.connection != nil {
		c.connection.Close()
		c.connection = nil
	}

	c.connected = false
}
