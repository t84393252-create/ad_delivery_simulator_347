package campaign

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ad-delivery-simulator/internal/models"
	"github.com/ad-delivery-simulator/pkg/kafka"
	"github.com/ad-delivery-simulator/pkg/redis"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type Service struct {
	db       *sql.DB
	redis    *redis.Client
	kafka    *kafka.Producer
	logger   *logrus.Logger
	brokers  []string
}

func NewService(db *sql.DB, redisClient *redis.Client, kafkaProducer *kafka.Producer, brokers []string, logger *logrus.Logger) *Service {
	return &Service{
		db:      db,
		redis:   redisClient,
		kafka:   kafkaProducer,
		brokers: brokers,
		logger:  logger,
	}
}

func (s *Service) CreateCampaign(ctx context.Context, campaign *models.Campaign) error {
	campaign.ID = uuid.New()
	campaign.CreatedAt = time.Now()
	campaign.UpdatedAt = time.Now()
	campaign.Status = models.CampaignStatusDraft
	campaign.SpentDaily = 0
	campaign.SpentTotal = 0

	query := `
		INSERT INTO campaigns (
			id, name, advertiser_id, status, budget_daily, budget_total,
			spent_daily, spent_total, bid_type, bid_amount, targeting_rules,
			frequency_capping, start_date, end_date, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`

	targetingJSON, _ := json.Marshal(campaign.TargetingRules)
	frequencyJSON, _ := json.Marshal(campaign.FrequencyCapping)

	_, err := s.db.ExecContext(ctx, query,
		campaign.ID, campaign.Name, campaign.AdvertiserID, campaign.Status,
		campaign.BudgetDaily, campaign.BudgetTotal, campaign.SpentDaily, campaign.SpentTotal,
		campaign.BidType, campaign.BidAmount, targetingJSON, frequencyJSON,
		campaign.StartDate, campaign.EndDate, campaign.CreatedAt, campaign.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create campaign: %w", err)
	}

	if err := s.redis.SetCampaignBudget(campaign.ID.String(), campaign.BudgetDaily, campaign.BudgetTotal); err != nil {
		s.logger.WithError(err).Error("Failed to set campaign budget in Redis")
	}

	s.publishCampaignUpdate(ctx, campaign, "created")

	return nil
}

func (s *Service) GetCampaign(ctx context.Context, campaignID uuid.UUID) (*models.Campaign, error) {
	query := `
		SELECT id, name, advertiser_id, status, budget_daily, budget_total,
			spent_daily, spent_total, bid_type, bid_amount, targeting_rules,
			frequency_capping, start_date, end_date, created_at, updated_at
		FROM campaigns WHERE id = $1
	`

	campaign := &models.Campaign{}
	var targetingJSON, frequencyJSON []byte
	var endDate sql.NullTime

	err := s.db.QueryRowContext(ctx, query, campaignID).Scan(
		&campaign.ID, &campaign.Name, &campaign.AdvertiserID, &campaign.Status,
		&campaign.BudgetDaily, &campaign.BudgetTotal, &campaign.SpentDaily, &campaign.SpentTotal,
		&campaign.BidType, &campaign.BidAmount, &targetingJSON, &frequencyJSON,
		&campaign.StartDate, &endDate, &campaign.CreatedAt, &campaign.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("campaign not found")
		}
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}

	if endDate.Valid {
		campaign.EndDate = &endDate.Time
	}

	if len(targetingJSON) > 0 {
		json.Unmarshal(targetingJSON, &campaign.TargetingRules)
	}
	if len(frequencyJSON) > 0 {
		json.Unmarshal(frequencyJSON, &campaign.FrequencyCapping)
	}

	return campaign, nil
}

func (s *Service) UpdateCampaign(ctx context.Context, campaign *models.Campaign) error {
	campaign.UpdatedAt = time.Now()

	query := `
		UPDATE campaigns SET
			name = $2, status = $3, budget_daily = $4, budget_total = $5,
			bid_type = $6, bid_amount = $7, targeting_rules = $8,
			frequency_capping = $9, end_date = $10, updated_at = $11
		WHERE id = $1
	`

	targetingJSON, _ := json.Marshal(campaign.TargetingRules)
	frequencyJSON, _ := json.Marshal(campaign.FrequencyCapping)

	_, err := s.db.ExecContext(ctx, query,
		campaign.ID, campaign.Name, campaign.Status, campaign.BudgetDaily, campaign.BudgetTotal,
		campaign.BidType, campaign.BidAmount, targetingJSON, frequencyJSON,
		campaign.EndDate, campaign.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update campaign: %w", err)
	}

	if err := s.redis.SetCampaignBudget(campaign.ID.String(), campaign.BudgetDaily, campaign.BudgetTotal); err != nil {
		s.logger.WithError(err).Error("Failed to update campaign budget in Redis")
	}

	s.publishCampaignUpdate(ctx, campaign, "updated")

	return nil
}

func (s *Service) ListActiveCampaigns(ctx context.Context) ([]*models.Campaign, error) {
	query := `
		SELECT id, name, advertiser_id, status, budget_daily, budget_total,
			spent_daily, spent_total, bid_type, bid_amount, targeting_rules,
			frequency_capping, start_date, end_date, created_at, updated_at
		FROM campaigns 
		WHERE status = $1 
			AND start_date <= NOW() 
			AND (end_date IS NULL OR end_date > NOW())
			AND spent_total < budget_total
			AND spent_daily < budget_daily
	`

	rows, err := s.db.QueryContext(ctx, query, models.CampaignStatusActive)
	if err != nil {
		return nil, fmt.Errorf("failed to list active campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []*models.Campaign
	for rows.Next() {
		campaign := &models.Campaign{}
		var targetingJSON, frequencyJSON []byte
		var endDate sql.NullTime

		err := rows.Scan(
			&campaign.ID, &campaign.Name, &campaign.AdvertiserID, &campaign.Status,
			&campaign.BudgetDaily, &campaign.BudgetTotal, &campaign.SpentDaily, &campaign.SpentTotal,
			&campaign.BidType, &campaign.BidAmount, &targetingJSON, &frequencyJSON,
			&campaign.StartDate, &endDate, &campaign.CreatedAt, &campaign.UpdatedAt,
		)

		if err != nil {
			s.logger.WithError(err).Error("Failed to scan campaign")
			continue
		}

		if endDate.Valid {
			campaign.EndDate = &endDate.Time
		}

		if len(targetingJSON) > 0 {
			json.Unmarshal(targetingJSON, &campaign.TargetingRules)
		}
		if len(frequencyJSON) > 0 {
			json.Unmarshal(frequencyJSON, &campaign.FrequencyCapping)
		}

		campaigns = append(campaigns, campaign)
	}

	return campaigns, nil
}

func (s *Service) CheckAndDecrementBudget(ctx context.Context, campaignID uuid.UUID, amount float64) (bool, error) {
	allowed, err := s.redis.DecrementBudget(campaignID.String(), amount)
	if err != nil {
		return false, fmt.Errorf("failed to decrement budget: %w", err)
	}

	if !allowed {
		s.logger.WithFields(logrus.Fields{
			"campaign_id": campaignID,
			"amount":      amount,
		}).Debug("Budget check failed")
		return false, nil
	}

	go s.updateSpentInDB(context.Background(), campaignID, amount)

	return true, nil
}

func (s *Service) updateSpentInDB(ctx context.Context, campaignID uuid.UUID, amount float64) {
	query := `
		UPDATE campaigns 
		SET spent_daily = spent_daily + $2, 
		    spent_total = spent_total + $2,
		    updated_at = NOW()
		WHERE id = $1
	`

	if _, err := s.db.ExecContext(ctx, query, campaignID, amount); err != nil {
		s.logger.WithError(err).WithField("campaign_id", campaignID).Error("Failed to update spent in database")
	}
}

func (s *Service) CheckFrequencyCap(ctx context.Context, userID string, campaignID uuid.UUID, eventType string) (bool, error) {
	campaign, err := s.GetCampaign(ctx, campaignID)
	if err != nil {
		return false, err
	}

	if campaign.FrequencyCapping == nil {
		return true, nil
	}

	count, err := s.redis.GetFrequencyCount(userID, campaignID.String(), eventType)
	if err != nil {
		return false, fmt.Errorf("failed to get frequency count: %w", err)
	}

	var cap int
	if eventType == "impression" {
		cap = campaign.FrequencyCapping.ImpressionCap
	} else if eventType == "click" {
		cap = campaign.FrequencyCapping.ClickCap
	}

	if cap > 0 && count >= int64(cap) {
		return false, nil
	}

	return true, nil
}

func (s *Service) IncrementFrequencyCap(ctx context.Context, userID string, campaignID uuid.UUID, eventType string) error {
	campaign, err := s.GetCampaign(ctx, campaignID)
	if err != nil {
		return err
	}

	if campaign.FrequencyCapping == nil {
		return nil
	}

	_, err = s.redis.IncrementFrequencyCap(userID, campaignID.String(), eventType, campaign.FrequencyCapping.TimeWindow)
	return err
}

func (s *Service) CalculatePacingRate(ctx context.Context, campaignID uuid.UUID) (float64, error) {
	campaign, err := s.GetCampaign(ctx, campaignID)
	if err != nil {
		return 1.0, err
	}

	now := time.Now()
	dayProgress := float64(now.Hour()*60+now.Minute()) / (24.0 * 60.0)
	
	budgetProgress := campaign.SpentDaily / campaign.BudgetDaily
	
	if budgetProgress > dayProgress*1.2 {
		return 0.5, nil
	} else if budgetProgress > dayProgress {
		return 0.8, nil
	}
	
	return 1.0, nil
}

func (s *Service) GetCampaignMetrics(ctx context.Context, campaignID uuid.UUID, date string) (*models.CampaignMetrics, error) {
	impressions, _ := s.redis.GetMetrics("impressions", campaignID.String(), date)
	clicks, _ := s.redis.GetMetrics("clicks", campaignID.String(), date)
	conversions, _ := s.redis.GetMetrics("conversions", campaignID.String(), date)
	
	campaign, err := s.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}

	metrics := &models.CampaignMetrics{
		CampaignID:  campaignID,
		Impressions: impressions,
		Clicks:      clicks,
		Conversions: conversions,
		Spend:       campaign.SpentDaily,
		Date:        time.Now(),
	}

	if impressions > 0 {
		metrics.CTR = float64(clicks) / float64(impressions) * 100
		metrics.CPM = (campaign.SpentDaily / float64(impressions)) * 1000
	}
	
	if clicks > 0 {
		metrics.CPC = campaign.SpentDaily / float64(clicks)
	}

	return metrics, nil
}

func (s *Service) ResetDailyBudgets(ctx context.Context) error {
	query := `UPDATE campaigns SET spent_daily = 0 WHERE status = $1`
	_, err := s.db.ExecContext(ctx, query, models.CampaignStatusActive)
	if err != nil {
		return fmt.Errorf("failed to reset daily budgets: %w", err)
	}

	campaigns, err := s.ListActiveCampaigns(ctx)
	if err != nil {
		return err
	}

	for _, campaign := range campaigns {
		if err := s.redis.SetCampaignBudget(campaign.ID.String(), campaign.BudgetDaily, campaign.BudgetTotal-campaign.SpentTotal); err != nil {
			s.logger.WithError(err).WithField("campaign_id", campaign.ID).Error("Failed to reset budget in Redis")
		}
	}

	return nil
}

func (s *Service) publishCampaignUpdate(ctx context.Context, campaign *models.Campaign, action string) {
	event := map[string]interface{}{
		"action":     action,
		"campaign":   campaign,
		"timestamp":  time.Now(),
	}

	if err := s.kafka.PublishCampaignUpdate(ctx, s.brokers, event); err != nil {
		s.logger.WithError(err).WithField("campaign_id", campaign.ID).Error("Failed to publish campaign update")
	}
}