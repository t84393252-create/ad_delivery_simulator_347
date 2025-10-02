package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

type Producer struct {
	writers map[string]*kafka.Writer
	logger  *logrus.Logger
}

func NewProducer(brokers []string, logger *logrus.Logger) *Producer {
	return &Producer{
		writers: make(map[string]*kafka.Writer),
		logger:  logger,
	}
}

func (p *Producer) GetWriter(topic string, brokers []string) *kafka.Writer {
	if writer, exists := p.writers[topic]; exists {
		return writer
	}

	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
		Async:    true,
		Compression: kafka.Snappy,
	}

	p.writers[topic] = writer
	return writer
}

func (p *Producer) PublishBidRequest(ctx context.Context, brokers []string, request interface{}) error {
	writer := p.GetWriter("bid-requests", brokers)
	
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal bid request: %w", err)
	}

	msg := kafka.Message{
		Value: data,
	}

	if err := writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to publish bid request: %w", err)
	}

	p.logger.Debug("Published bid request to Kafka")
	return nil
}

func (p *Producer) PublishBidResponse(ctx context.Context, brokers []string, response interface{}) error {
	writer := p.GetWriter("bid-responses", brokers)
	
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal bid response: %w", err)
	}

	msg := kafka.Message{
		Value: data,
	}

	if err := writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to publish bid response: %w", err)
	}

	p.logger.Debug("Published bid response to Kafka")
	return nil
}

func (p *Producer) PublishImpression(ctx context.Context, brokers []string, impression interface{}) error {
	writer := p.GetWriter("impressions", brokers)
	
	data, err := json.Marshal(impression)
	if err != nil {
		return fmt.Errorf("failed to marshal impression: %w", err)
	}

	msg := kafka.Message{
		Value: data,
	}

	if err := writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to publish impression: %w", err)
	}

	p.logger.Debug("Published impression to Kafka")
	return nil
}

func (p *Producer) PublishClick(ctx context.Context, brokers []string, click interface{}) error {
	writer := p.GetWriter("clicks", brokers)
	
	data, err := json.Marshal(click)
	if err != nil {
		return fmt.Errorf("failed to marshal click: %w", err)
	}

	msg := kafka.Message{
		Value: data,
	}

	if err := writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to publish click: %w", err)
	}

	p.logger.Debug("Published click to Kafka")
	return nil
}

func (p *Producer) PublishCampaignUpdate(ctx context.Context, brokers []string, update interface{}) error {
	writer := p.GetWriter("campaign-updates", brokers)
	
	data, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal campaign update: %w", err)
	}

	msg := kafka.Message{
		Value: data,
	}

	if err := writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to publish campaign update: %w", err)
	}

	p.logger.Debug("Published campaign update to Kafka")
	return nil
}

func (p *Producer) PublishEvent(ctx context.Context, brokers []string, topic string, event interface{}) error {
	writer := p.GetWriter(topic, brokers)
	
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := kafka.Message{
		Value: data,
	}

	if err := writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to publish event to topic %s: %w", topic, err)
	}

	p.logger.WithField("topic", topic).Debug("Published event to Kafka")
	return nil
}

func (p *Producer) Close() error {
	for topic, writer := range p.writers {
		if err := writer.Close(); err != nil {
			p.logger.WithError(err).WithField("topic", topic).Error("Failed to close Kafka writer")
		}
	}
	return nil
}