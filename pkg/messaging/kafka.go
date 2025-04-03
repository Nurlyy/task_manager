package messaging

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/nurlyy/task_manager/pkg/config"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// KafkaProducer представляет клиент для отправки сообщений в Kafka
type KafkaProducer struct {
	Writer  *kafka.Writer
	Config  *config.KafkaConfig
	Logger  logger.Logger
}

// KafkaConsumer представляет клиент для чтения сообщений из Kafka
type KafkaConsumer struct {
	Reader  *kafka.Reader
	Config  *config.KafkaConfig
	Logger  logger.Logger
}

// NewKafkaProducer создает нового производителя Kafka
func NewKafkaProducer(cfg *config.KafkaConfig, log logger.Logger) *KafkaProducer {
	log.Info("Creating Kafka producer", map[string]interface{}{
		"brokers": cfg.Brokers,
	})

	writer := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Brokers...),
		Balancer: &kafka.LeastBytes{},
		// Настройки для надежной доставки
		RequiredAcks: kafka.RequireAll,
		Async:        false,
		// Настройки для повторных попыток
		MaxAttempts:   5,
		RetryBackoff:  time.Millisecond * 250,
		// Настройки для производительности
		BatchSize:    100,
		BatchTimeout: time.Millisecond * 10,
		// Сжатие сообщений
		CompressionCodec: kafka.Snappy,
	}

	return &KafkaProducer{
		Writer: writer,
		Config: cfg,
		Logger: log,
	}
}

// Close закрывает соединение производителя
func (p *KafkaProducer) Close() error {
	p.Logger.Info("Closing Kafka producer")
	return p.Writer.Close()
}

// Publish отправляет сообщение в указанный топик
func (p *KafkaProducer) Publish(ctx context.Context, topic string, key string, value []byte) error {
	p.Writer.Topic = topic

	start := time.Now()
	err := p.Writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: value,
		Time:  time.Now(),
	})
	elapsed := time.Since(start)

	if err != nil {
		p.Logger.Error("Failed to publish Kafka message", err, map[string]interface{}{
			"topic":   topic,
			"key":     key,
			"elapsed": elapsed.String(),
		})
		return fmt.Errorf("failed to publish Kafka message to topic %s: %w", topic, err)
	}

	p.Logger.Debug("Successfully published Kafka message", map[string]interface{}{
		"topic":   topic,
		"key":     key,
		"elapsed": elapsed.String(),
	})
	return nil
}

// NewKafkaConsumer создает нового потребителя Kafka
func NewKafkaConsumer(topic, groupID string, cfg *config.KafkaConfig, log logger.Logger) *KafkaConsumer {
	log.Info("Creating Kafka consumer", map[string]interface{}{
		"brokers": cfg.Brokers,
		"topic":   topic,
		"groupID": groupID,
	})

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10e3,    // 10KB
		MaxBytes:       10e6,    // 10MB
		MaxWait:        time.Second,
		StartOffset:    kafka.FirstOffset,
		CommitInterval: time.Second,
		RetentionTime:  7 * 24 * time.Hour, // 1 неделя
		
		// Настройки для повторных попыток и таймаутов
		ReadBackoffMin: time.Millisecond * 100,
		ReadBackoffMax: time.Second * 1,
		
		// Если необходимо использовать TLS
		// Диагностика производительности
		ReadLagInterval: time.Minute,
	})

	return &KafkaConsumer{
		Reader: reader,
		Config: cfg,
		Logger: log,
	}
}

// Close закрывает соединение потребителя
func (c *KafkaConsumer) Close() error {
	c.Logger.Info("Closing Kafka consumer")
	return c.Reader.Close()
}

// Read читает следующее сообщение из Kafka
func (c *KafkaConsumer) Read(ctx context.Context) (kafka.Message, error) {
	start := time.Now()
	message, err := c.Reader.ReadMessage(ctx)
	elapsed := time.Since(start)

	if err != nil {
		c.Logger.Error("Failed to read Kafka message", err, map[string]interface{}{
			"topic":   c.Reader.Config().Topic,
			"groupID": c.Reader.Config().GroupID,
			"elapsed": elapsed.String(),
		})
		return kafka.Message{}, fmt.Errorf("failed to read Kafka message: %w", err)
	}

	c.Logger.Debug("Successfully read Kafka message", map[string]interface{}{
		"topic":   c.Reader.Config().Topic,
		"groupID": c.Reader.Config().GroupID,
		"key":     string(message.Key),
		"elapsed": elapsed.String(),
	})
	return message, nil
}

// CommitMessages коммитит сообщения для подтверждения обработки
func (c *KafkaConsumer) CommitMessages(ctx context.Context, messages ...kafka.Message) error {
	if err := c.Reader.CommitMessages(ctx, messages...); err != nil {
		c.Logger.Error("Failed to commit Kafka messages", err, map[string]interface{}{
			"topic":   c.Reader.Config().Topic,
			"groupID": c.Reader.Config().GroupID,
			"count":   len(messages),
		})
		return fmt.Errorf("failed to commit Kafka messages: %w", err)
	}

	c.Logger.Debug("Successfully committed Kafka messages", map[string]interface{}{
		"topic":   c.Reader.Config().Topic,
		"groupID": c.Reader.Config().GroupID,
		"count":   len(messages),
	})
	return nil
}

// CreateTopics создает топики в Kafka, если они не существуют
func CreateTopics(ctx context.Context, brokers []string, topics []string, log logger.Logger) error {
	log.Info("Creating Kafka topics", map[string]interface{}{
		"brokers": brokers,
		"topics":  topics,
	})

	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get Kafka controller: %w", err)
	}

	controllerConn, err := kafka.DialContext(ctx, "tcp", controller.Host)
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka controller: %w", err)
	}
	defer controllerConn.Close()

	topicConfigs := make([]kafka.TopicConfig, 0, len(topics))
	for _, topic := range topics {
		topicConfigs = append(topicConfigs, kafka.TopicConfig{
			Topic:             topic,
			NumPartitions:     3,
			ReplicationFactor: 1,
			ConfigEntries: []kafka.ConfigEntry{
				{
					ConfigName:  "retention.ms",
					ConfigValue: "604800000", // 7 дней
				},
			},
		})
	}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		log.Error("Failed to create Kafka topics", err, map[string]interface{}{
			"topics": topics,
		})
		return fmt.Errorf("failed to create Kafka topics: %w", err)
	}

	log.Info("Kafka topics created successfully", map[string]interface{}{
		"topics": topics,
	})
	return nil
}