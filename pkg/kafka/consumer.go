package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

type Consumer struct {
	readers map[string]*kafka.Reader
	logger  *logrus.Logger
}

type MessageHandler func(ctx context.Context, message []byte) error

func NewConsumer(logger *logrus.Logger) *Consumer {
	return &Consumer{
		readers: make(map[string]*kafka.Reader),
		logger:  logger,
	}
}

func (c *Consumer) CreateReader(topic string, brokers []string, groupID string) *kafka.Reader {
	if reader, exists := c.readers[topic]; exists {
		return reader
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       topic,
		GroupID:     groupID,
		MinBytes:    10e3,
		MaxBytes:    10e6,
		StartOffset: kafka.LastOffset,
	})

	c.readers[topic] = reader
	return reader
}

func (c *Consumer) ConsumeBidRequests(ctx context.Context, brokers []string, groupID string, handler MessageHandler) error {
	reader := c.CreateReader("bid-requests", brokers, groupID)
	return c.consumeMessages(ctx, reader, "bid-requests", handler)
}

func (c *Consumer) ConsumeBidResponses(ctx context.Context, brokers []string, groupID string, handler MessageHandler) error {
	reader := c.CreateReader("bid-responses", brokers, groupID)
	return c.consumeMessages(ctx, reader, "bid-responses", handler)
}

func (c *Consumer) ConsumeImpressions(ctx context.Context, brokers []string, groupID string, handler MessageHandler) error {
	reader := c.CreateReader("impressions", brokers, groupID)
	return c.consumeMessages(ctx, reader, "impressions", handler)
}

func (c *Consumer) ConsumeClicks(ctx context.Context, brokers []string, groupID string, handler MessageHandler) error {
	reader := c.CreateReader("clicks", brokers, groupID)
	return c.consumeMessages(ctx, reader, "clicks", handler)
}

func (c *Consumer) ConsumeCampaignUpdates(ctx context.Context, brokers []string, groupID string, handler MessageHandler) error {
	reader := c.CreateReader("campaign-updates", brokers, groupID)
	return c.consumeMessages(ctx, reader, "campaign-updates", handler)
}

func (c *Consumer) ConsumeFromTopic(ctx context.Context, topic string, brokers []string, groupID string, handler MessageHandler) error {
	reader := c.CreateReader(topic, brokers, groupID)
	return c.consumeMessages(ctx, reader, topic, handler)
}

func (c *Consumer) consumeMessages(ctx context.Context, reader *kafka.Reader, topic string, handler MessageHandler) error {
	for {
		select {
		case <-ctx.Done():
			c.logger.WithField("topic", topic).Info("Stopping consumer due to context cancellation")
			return ctx.Err()
		default:
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				if err == context.Canceled {
					return nil
				}
				c.logger.WithError(err).WithField("topic", topic).Error("Failed to read message")
				continue
			}

			if err := handler(ctx, msg.Value); err != nil {
				c.logger.WithError(err).WithField("topic", topic).Error("Failed to process message")
				continue
			}

			c.logger.WithFields(logrus.Fields{
				"topic":     topic,
				"partition": msg.Partition,
				"offset":    msg.Offset,
			}).Debug("Successfully processed message")
		}
	}
}

func (c *Consumer) ProcessBidRequest(ctx context.Context, data []byte, processor func(request interface{}) error) error {
	var request map[string]interface{}
	if err := json.Unmarshal(data, &request); err != nil {
		return fmt.Errorf("failed to unmarshal bid request: %w", err)
	}
	return processor(request)
}

func (c *Consumer) ProcessImpression(ctx context.Context, data []byte, processor func(impression interface{}) error) error {
	var impression map[string]interface{}
	if err := json.Unmarshal(data, &impression); err != nil {
		return fmt.Errorf("failed to unmarshal impression: %w", err)
	}
	return processor(impression)
}

func (c *Consumer) ProcessClick(ctx context.Context, data []byte, processor func(click interface{}) error) error {
	var click map[string]interface{}
	if err := json.Unmarshal(data, &click); err != nil {
		return fmt.Errorf("failed to unmarshal click: %w", err)
	}
	return processor(click)
}

func (c *Consumer) Close() error {
	for topic, reader := range c.readers {
		if err := reader.Close(); err != nil {
			c.logger.WithError(err).WithField("topic", topic).Error("Failed to close Kafka reader")
		}
	}
	return nil
}

type BatchConsumer struct {
	*Consumer
	batchSize int
}

func NewBatchConsumer(logger *logrus.Logger, batchSize int) *BatchConsumer {
	return &BatchConsumer{
		Consumer:  NewConsumer(logger),
		batchSize: batchSize,
	}
}

func (bc *BatchConsumer) ConsumeBatch(ctx context.Context, topic string, brokers []string, groupID string, handler func(messages [][]byte) error) error {
	reader := bc.CreateReader(topic, brokers, groupID)
	
	batch := make([][]byte, 0, bc.batchSize)
	
	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				if err := handler(batch); err != nil {
					bc.logger.WithError(err).Error("Failed to process final batch")
				}
			}
			return ctx.Err()
		default:
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				if err == context.Canceled {
					if len(batch) > 0 {
						if err := handler(batch); err != nil {
							bc.logger.WithError(err).Error("Failed to process final batch")
						}
					}
					return nil
				}
				bc.logger.WithError(err).Error("Failed to read message")
				continue
			}

			batch = append(batch, msg.Value)

			if len(batch) >= bc.batchSize {
				if err := handler(batch); err != nil {
					bc.logger.WithError(err).Error("Failed to process batch")
				}
				batch = make([][]byte, 0, bc.batchSize)
			}
		}
	}
}