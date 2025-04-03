package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// KafkaConsumer реализует интерфейс потребителя для получения сообщений из Kafka
type KafkaConsumer struct {
	reader  *kafka.Reader
	logger  logger.Logger
}

// NewKafkaConsumer создает новый экземпляр KafkaConsumer
func NewKafkaConsumer(brokers []string, topic, groupID string, logger logger.Logger) *KafkaConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		MaxWait:        1 * time.Second,
		StartOffset:    kafka.FirstOffset,
		CommitInterval: 1 * time.Second,
		RetentionTime:  7 * 24 * time.Hour, // 1 week
		Logger:         kafka.LoggerFunc(logger.Debug),
	})

	return &KafkaConsumer{
		reader: reader,
		logger: logger,
	}
}

// Close закрывает соединение с Kafka
func (c *KafkaConsumer) Close() error {
	c.logger.Info("Closing Kafka consumer")
	return c.reader.Close()
}

// ReadMessage читает сообщение из Kafka
func (c *KafkaConsumer) ReadMessage(ctx context.Context) (*Message, error) {
	start := time.Now()
	kafkaMsg, err := c.reader.ReadMessage(ctx)
	elapsed := time.Since(start)

	if err != nil {
		c.logger.Error("Failed to read message", err, map[string]interface{}{
			"topic":   c.reader.Config().Topic,
			"group":   c.reader.Config().GroupID,
			"elapsed": elapsed.String(),
		})
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	c.logger.Debug("Successfully read message", map[string]interface{}{
		"topic":   c.reader.Config().Topic,
		"group":   c.reader.Config().GroupID,
		"key":     string(kafkaMsg.Key),
		"elapsed": elapsed.String(),
	})

	return &Message{
		Key:       string(kafkaMsg.Key),
		Value:     kafkaMsg.Value,
		Topic:     kafkaMsg.Topic,
		Partition: kafkaMsg.Partition,
		Offset:    kafkaMsg.Offset,
		Time:      kafkaMsg.Time,
		Raw:       kafkaMsg,
	}, nil
}

// CommitMessages подтверждает обработку сообщений
func (c *KafkaConsumer) CommitMessages(ctx context.Context, msgs ...Message) error {
	if len(msgs) == 0 {
		return nil
	}

	kafkaMsgs := make([]kafka.Message, len(msgs))
	for i, msg := range msgs {
		kafkaMsgs[i] = msg.Raw
	}

	if err := c.reader.CommitMessages(ctx, kafkaMsgs...); err != nil {
		c.logger.Error("Failed to commit messages", err, map[string]interface{}{
			"topic": c.reader.Config().Topic,
			"group": c.reader.Config().GroupID,
			"count": len(msgs),
		})
		return fmt.Errorf("failed to commit messages: %w", err)
	}

	c.logger.Debug("Successfully committed messages", map[string]interface{}{
		"topic": c.reader.Config().Topic,
		"group": c.reader.Config().GroupID,
		"count": len(msgs),
	})

	return nil
}

// ParseMessage десериализует сообщение в структуру
func (c *KafkaConsumer) ParseMessage(msg *Message, dest interface{}) error {
	if err := json.Unmarshal(msg.Value, dest); err != nil {
		c.logger.Error("Failed to parse message", err, map[string]interface{}{
			"topic": msg.Topic,
			"key":   msg.Key,
		})
		return fmt.Errorf("failed to parse message: %w", err)
	}

	return nil
}

// Message представляет сообщение из Kafka
type Message struct {
	Key       string
	Value     []byte
	Topic     string
	Partition int
	Offset    int64
	Time      time.Time
	Raw       kafka.Message
}