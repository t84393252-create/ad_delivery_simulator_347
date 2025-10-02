package tracking

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ad-delivery-simulator/internal/campaign"
	"github.com/ad-delivery-simulator/internal/models"
	"github.com/ad-delivery-simulator/pkg/kafka"
	"github.com/ad-delivery-simulator/pkg/redis"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
)

var (
	impressionCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ad_impressions_total",
		Help: "Total number of ad impressions",
	}, []string{"campaign_id"})

	clickCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ad_clicks_total",
		Help: "Total number of ad clicks",
	}, []string{"campaign_id"})

	conversionCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ad_conversions_total",
		Help: "Total number of ad conversions",
	}, []string{"campaign_id"})

	trackingLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "tracking_processing_duration_seconds",
		Help:    "Time taken to process tracking events",
		Buckets: prometheus.DefBuckets,
	}, []string{"event_type"})
)

type Service struct {
	db              *sql.DB
	redis           *redis.Client
	kafka           *kafka.Producer
	campaignService *campaign.Service
	brokers         []string
	logger          *logrus.Logger
	eventBuffer     chan *models.TrackingEvent
	bufferSize      int
	workerPool      int
	wg              sync.WaitGroup
}

func NewService(
	db *sql.DB,
	redisClient *redis.Client,
	kafkaProducer *kafka.Producer,
	campaignService *campaign.Service,
	brokers []string,
	logger *logrus.Logger,
) *Service {
	return &Service{
		db:              db,
		redis:           redisClient,
		kafka:           kafkaProducer,
		campaignService: campaignService,
		brokers:         brokers,
		logger:          logger,
		eventBuffer:     make(chan *models.TrackingEvent, 10000),
		bufferSize:      10000,
		workerPool:      10,
	}
}

func (s *Service) Start(ctx context.Context) {
	for i := 0; i < s.workerPool; i++ {
		s.wg.Add(1)
		go s.processEventWorker(ctx)
	}

	go s.batchProcessor(ctx)
}

func (s *Service) Stop() {
	close(s.eventBuffer)
	s.wg.Wait()
}

func (s *Service) TrackImpression(ctx context.Context, event *models.TrackingEvent) error {
	timer := prometheus.NewTimer(trackingLatency.WithLabelValues("impression"))
	defer timer.ObserveDuration()

	event.ID = uuid.New()
	event.Type = models.EventTypeImpression
	event.Timestamp = time.Now()

	if err := s.validateAndEnrichEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to validate impression event: %w", err)
	}

	impressionCounter.WithLabelValues(event.CampaignID.String()).Inc()

	if err := s.redis.IncrementMetric("impressions", event.CampaignID.String()); err != nil {
		s.logger.WithError(err).Error("Failed to increment impression metric in Redis")
	}

	if event.UserID != "" {
		if err := s.campaignService.IncrementFrequencyCap(ctx, event.UserID, event.CampaignID, "impression"); err != nil {
			s.logger.WithError(err).Error("Failed to increment frequency cap")
		}
	}

	select {
	case s.eventBuffer <- event:
	default:
		s.logger.Warn("Event buffer full, processing synchronously")
		if err := s.processEvent(ctx, event); err != nil {
			return err
		}
	}

	if err := s.kafka.PublishImpression(ctx, s.brokers, event); err != nil {
		s.logger.WithError(err).Error("Failed to publish impression to Kafka")
	}

	return nil
}

func (s *Service) TrackClick(ctx context.Context, event *models.TrackingEvent) error {
	timer := prometheus.NewTimer(trackingLatency.WithLabelValues("click"))
	defer timer.ObserveDuration()

	event.ID = uuid.New()
	event.Type = models.EventTypeClick
	event.Timestamp = time.Now()

	if err := s.validateAndEnrichEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to validate click event: %w", err)
	}

	clickCounter.WithLabelValues(event.CampaignID.String()).Inc()

	if err := s.redis.IncrementMetric("clicks", event.CampaignID.String()); err != nil {
		s.logger.WithError(err).Error("Failed to increment click metric in Redis")
	}

	if event.UserID != "" {
		if err := s.campaignService.IncrementFrequencyCap(ctx, event.UserID, event.CampaignID, "click"); err != nil {
			s.logger.WithError(err).Error("Failed to increment frequency cap")
		}
	}

	select {
	case s.eventBuffer <- event:
	default:
		s.logger.Warn("Event buffer full, processing synchronously")
		if err := s.processEvent(ctx, event); err != nil {
			return err
		}
	}

	if err := s.kafka.PublishClick(ctx, s.brokers, event); err != nil {
		s.logger.WithError(err).Error("Failed to publish click to Kafka")
	}

	return nil
}

func (s *Service) TrackConversion(ctx context.Context, event *models.TrackingEvent) error {
	timer := prometheus.NewTimer(trackingLatency.WithLabelValues("conversion"))
	defer timer.ObserveDuration()

	event.ID = uuid.New()
	event.Type = models.EventTypeConversion
	event.Timestamp = time.Now()

	if err := s.validateAndEnrichEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to validate conversion event: %w", err)
	}

	conversionCounter.WithLabelValues(event.CampaignID.String()).Inc()

	if err := s.redis.IncrementMetric("conversions", event.CampaignID.String()); err != nil {
		s.logger.WithError(err).Error("Failed to increment conversion metric in Redis")
	}

	select {
	case s.eventBuffer <- event:
	default:
		s.logger.Warn("Event buffer full, processing synchronously")
		if err := s.processEvent(ctx, event); err != nil {
			return err
		}
	}

	if err := s.kafka.PublishEvent(ctx, s.brokers, "conversions", event); err != nil {
		s.logger.WithError(err).Error("Failed to publish conversion to Kafka")
	}

	return nil
}

func (s *Service) validateAndEnrichEvent(ctx context.Context, event *models.TrackingEvent) error {
	if event.CampaignID == uuid.Nil {
		return fmt.Errorf("invalid campaign ID")
	}

	campaign, err := s.campaignService.GetCampaign(ctx, event.CampaignID)
	if err != nil {
		return fmt.Errorf("campaign not found: %w", err)
	}

	if campaign.Status != models.CampaignStatusActive {
		return fmt.Errorf("campaign is not active")
	}

	if event.Type == models.EventTypeClick || event.Type == models.EventTypeConversion {
		event.Price = campaign.BidAmount
		
		if campaign.BidType == models.BidTypeCPC && event.Type == models.EventTypeClick {
			if allowed, err := s.campaignService.CheckAndDecrementBudget(ctx, campaign.ID, event.Price); err != nil || !allowed {
				return fmt.Errorf("budget exceeded for CPC campaign")
			}
		} else if campaign.BidType == models.BidTypeCPA && event.Type == models.EventTypeConversion {
			if allowed, err := s.campaignService.CheckAndDecrementBudget(ctx, campaign.ID, event.Price); err != nil || !allowed {
				return fmt.Errorf("budget exceeded for CPA campaign")
			}
		}
	}

	return nil
}

func (s *Service) processEvent(ctx context.Context, event *models.TrackingEvent) error {
	query := `
		INSERT INTO tracking_events (
			id, type, campaign_id, creative_id, user_id, session_id,
			ip, user_agent, referrer, price, timestamp, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	metadataJSON, _ := json.Marshal(event.Metadata)

	_, err := s.db.ExecContext(ctx, query,
		event.ID, event.Type, event.CampaignID, event.CreativeID,
		event.UserID, event.SessionID, event.IP, event.UserAgent,
		event.Referrer, event.Price, event.Timestamp, metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to insert tracking event: %w", err)
	}

	now := time.Now()
	event.ProcessedAt = &now

	return nil
}

func (s *Service) processEventWorker(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-s.eventBuffer:
			if !ok {
				return
			}
			if err := s.processEvent(ctx, event); err != nil {
				s.logger.WithError(err).Error("Failed to process event")
			}
		}
	}
}

func (s *Service) batchProcessor(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	batch := make([]*models.TrackingEvent, 0, 100)

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				s.processBatch(ctx, batch)
			}
			return
		case <-ticker.C:
			if len(batch) > 0 {
				s.processBatch(ctx, batch)
				batch = make([]*models.TrackingEvent, 0, 100)
			}
		case event := <-s.eventBuffer:
			batch = append(batch, event)
			if len(batch) >= 100 {
				s.processBatch(ctx, batch)
				batch = make([]*models.TrackingEvent, 0, 100)
			}
		}
	}
}

func (s *Service) processBatch(ctx context.Context, events []*models.TrackingEvent) {
	if len(events) == 0 {
		return
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.WithError(err).Error("Failed to begin transaction")
		return
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO tracking_events (
			id, type, campaign_id, creative_id, user_id, session_id,
			ip, user_agent, referrer, price, timestamp, metadata, processed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`)
	if err != nil {
		s.logger.WithError(err).Error("Failed to prepare statement")
		return
	}
	defer stmt.Close()

	now := time.Now()
	for _, event := range events {
		event.ProcessedAt = &now
		metadataJSON, _ := json.Marshal(event.Metadata)
		
		_, err := stmt.ExecContext(ctx,
			event.ID, event.Type, event.CampaignID, event.CreativeID,
			event.UserID, event.SessionID, event.IP, event.UserAgent,
			event.Referrer, event.Price, event.Timestamp, metadataJSON, event.ProcessedAt,
		)
		if err != nil {
			s.logger.WithError(err).Error("Failed to insert event in batch")
		}
	}

	if err := tx.Commit(); err != nil {
		s.logger.WithError(err).Error("Failed to commit batch")
	} else {
		s.logger.WithField("count", len(events)).Debug("Successfully processed batch")
	}
}

func (s *Service) GetEventStats(ctx context.Context, campaignID uuid.UUID, startTime, endTime time.Time) (map[string]int64, error) {
	query := `
		SELECT type, COUNT(*) as count
		FROM tracking_events
		WHERE campaign_id = $1 AND timestamp BETWEEN $2 AND $3
		GROUP BY type
	`

	rows, err := s.db.QueryContext(ctx, query, campaignID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get event stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int64)
	for rows.Next() {
		var eventType string
		var count int64
		if err := rows.Scan(&eventType, &count); err != nil {
			s.logger.WithError(err).Error("Failed to scan event stats")
			continue
		}
		stats[eventType] = count
	}

	return stats, nil
}

func (s *Service) GetRealTimeMetrics(ctx context.Context, campaignID string) (*models.CampaignMetrics, error) {
	date := time.Now().Format("2006-01-02")
	return s.campaignService.GetCampaignMetrics(ctx, uuid.MustParse(campaignID), date)
}