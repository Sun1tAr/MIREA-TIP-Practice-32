package rabbit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"github.com/sun1tar/MIREA-TIP-Practice-32/tech-ip-sem2/shared/models"
)

// ProcessedIDs хранилище обработанных message_id (в памяти для демо)
var processedIDs = struct {
	sync.RWMutex
	ids map[string]bool
}{ids: make(map[string]bool)}

type Consumer struct {
	conn          *amqp.Connection
	channel       *amqp.Channel
	mainQueue     string
	retryQueue    string
	dlq           string
	prefetchCount int
	maxAttempts   int
	retryDelay    int
	logger        *logrus.Logger
}

func NewConsumer(rabbitURL, mainQueue, retryQueue, dlq string, prefetchCount, maxAttempts, retryDelay int, logger *logrus.Logger) (*Consumer, error) {
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Настраиваем prefetch
	err = ch.Qos(
		prefetchCount,
		0,
		false,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	// Создаём всё необходимое: exchanges, очереди, привязки
	if err := setupInfrastructure(ch, mainQueue, retryQueue, dlq, retryDelay); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to setup infrastructure: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"main_queue":   mainQueue,
		"retry_queue":  retryQueue,
		"dlq":          dlq,
		"prefetch":     prefetchCount,
		"max_attempts": maxAttempts,
		"retry_delay":  retryDelay,
	}).Info("RabbitMQ consumer connected with DLQ and retry")

	return &Consumer{
		conn:          conn,
		channel:       ch,
		mainQueue:     mainQueue,
		retryQueue:    retryQueue,
		dlq:           dlq,
		prefetchCount: prefetchCount,
		maxAttempts:   maxAttempts,
		retryDelay:    retryDelay,
		logger:        logger,
	}, nil
}

// setupInfrastructure создаёт все необходимые элементы RabbitMQ в правильном порядке
func setupInfrastructure(ch *amqp.Channel, mainQueue, retryQueue, dlq string, retryDelay int) error {
	// 1. Сначала создаём основную очередь (без DLX, потом добавим)
	_, err := ch.QueueDeclare(
		mainQueue,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare main queue: %w", err)
	}

	// 2. Создаём DLX для retry очереди
	retryDLX := mainQueue + "_retry_dlx"
	err = ch.ExchangeDeclare(
		retryDLX,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare retry DLX: %w", err)
	}

	// 3. Создаём retry очередь с TTL и привязкой к DLX
	_, err = ch.QueueDeclare(
		retryQueue,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    retryDLX,
			"x-dead-letter-routing-key": mainQueue,
			"x-message-ttl":             int32(retryDelay * 1000), // в миллисекундах
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare retry queue: %w", err)
	}

	// 4. Привязываем retry DLX к mainQueue (теперь mainQueue уже существует)
	err = ch.QueueBind(
		mainQueue,
		mainQueue,
		retryDLX,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind main queue to retry DLX: %w", err)
	}

	// 5. Создаём DLX для основной очереди
	mainDLX := mainQueue + "_dlx"
	err = ch.ExchangeDeclare(
		mainDLX,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare main DLX: %w", err)
	}

	// 6. Обновляем основную очередь с DLX (удаляем и создаём заново с DLX)
	// Сначала удаляем старую версию (если она была без DLX)
	_, err = ch.QueueDelete(
		mainQueue,
		false, // if unused
		false, // if empty
		false, // no wait
	)
	if err != nil {
		// Игнорируем ошибку, если очередь не существует
	}

	// Создаём основную очередь с DLX
	_, err = ch.QueueDeclare(
		mainQueue,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    mainDLX,
			"x-dead-letter-routing-key": dlq,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare main queue with DLX: %w", err)
	}

	// 7. Создаём DLQ
	_, err = ch.QueueDeclare(
		dlq,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare DLQ: %w", err)
	}

	// 8. Привязываем DLQ к main DLX
	err = ch.QueueBind(
		dlq,
		dlq,
		mainDLX,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind DLQ to main DLX: %w", err)
	}

	// 9. Восстанавливаем привязку retry DLX к mainQueue (после пересоздания)
	err = ch.QueueBind(
		mainQueue,
		mainQueue,
		retryDLX,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to rebind main queue to retry DLX: %w", err)
	}

	return nil
}

func (c *Consumer) StartConsuming(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		c.mainQueue,
		"",
		false, // auto-ack = false
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	c.logger.Info("Worker started, waiting for jobs...")

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Stopping consumer...")
			return nil
		case msg := <-msgs:
			c.processMessage(msg)
		}
	}
}

// isProcessed проверяет, обрабатывалось ли сообщение
func isProcessed(messageID string) bool {
	processedIDs.RLock()
	defer processedIDs.RUnlock()
	return processedIDs.ids[messageID]
}

// markProcessed помечает сообщение как обработанное
func markProcessed(messageID string) {
	processedIDs.Lock()
	defer processedIDs.Unlock()
	processedIDs.ids[messageID] = true

	// В реальном проекте здесь была бы очистка старых ID
	// Но для демо оставляем как есть
}

func (c *Consumer) processMessage(msg amqp.Delivery) {
	logEntry := c.logger.WithFields(logrus.Fields{
		"delivery_tag": msg.DeliveryTag,
		"message_id":   msg.MessageId,
	})

	// Парсим сообщение
	var job models.JobMessage
	if err := json.Unmarshal(msg.Body, &job); err != nil {
		logEntry.WithError(err).Error("Failed to unmarshal job")
		// Невалидное сообщение - сразу в DLQ
		msg.Nack(false, false)
		return
	}

	// Устанавливаем MessageId из тела, если не заполнен
	if msg.MessageId == "" {
		msg.MessageId = job.MessageID
	}

	logEntry = logEntry.WithFields(logrus.Fields{
		"job":        job.Job,
		"task_id":    job.TaskID,
		"attempt":    job.Attempt,
		"message_id": job.MessageID,
	})

	// Проверка на дубликат (идемпотентность)
	if isProcessed(job.MessageID) {
		logEntry.Info("⏭️ Job already processed, skipping (idempotency)")
		msg.Ack(false)
		return
	}

	// Имитация обработки задачи
	logEntry.Info("🔄 Processing job...")

	// Имитация "тяжёлой" работы
	time.Sleep(2 * time.Second)

	// Имитация ошибки для демонстрации retry
	shouldFail := false
	if job.TaskID == "t_fail" || job.TaskID == "fail" {
		shouldFail = true
	}

	if shouldFail {
		logEntry.Warn("❌ Job failed, will retry if attempts left")

		// Увеличиваем счётчик попыток
		job.Attempt++

		if job.Attempt <= c.maxAttempts {
			// Отправляем в retry очередь
			if err := c.sendToRetryQueue(job); err != nil {
				logEntry.WithError(err).Error("Failed to send to retry queue")
				msg.Nack(false, false)
				return
			}
			logEntry.WithField("next_attempt", job.Attempt).Info("⏳ Job sent to retry queue")
			msg.Ack(false) // Подтверждаем исходное сообщение
		} else {
			// Превышено число попыток - отправляем в DLQ
			if err := c.sendToDLQ(job); err != nil {
				logEntry.WithError(err).Error("Failed to send to DLQ")
				msg.Nack(false, false)
				return
			}
			logEntry.Warn("💀 Max attempts reached, job sent to DLQ")
			msg.Ack(false)
		}
		return
	}

	// Успешная обработка
	logEntry.Info("✅ Job processed successfully")

	// Помечаем как обработанное (идемпотентность)
	markProcessed(job.MessageID)

	msg.Ack(false)
}

func (c *Consumer) sendToRetryQueue(job models.JobMessage) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	return c.channel.Publish(
		"",           // exchange (default)
		c.retryQueue, // routing key
		true,         // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    job.MessageID,
			Timestamp:    time.Now(),
			Body:         body,
			Headers: amqp.Table{
				"x-original-queue": c.mainQueue,
				"x-attempt":        job.Attempt,
			},
		},
	)
}

func (c *Consumer) sendToDLQ(job models.JobMessage) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	return c.channel.Publish(
		"",    // exchange (default)
		c.dlq, // routing key
		true,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    job.MessageID,
			Timestamp:    time.Now(),
			Body:         body,
			Headers: amqp.Table{
				"x-final-attempt": job.Attempt,
				"x-error":         "max_attempts_exceeded",
			},
		},
	)
}

func (c *Consumer) Close() error {
	var err error
	if c.channel != nil {
		if e := c.channel.Close(); e != nil {
			err = e
		}
	}
	if c.conn != nil {
		if e := c.conn.Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}
