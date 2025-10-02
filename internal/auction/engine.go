package auction

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/ad-delivery-simulator/internal/campaign"
	"github.com/ad-delivery-simulator/internal/models"
	"github.com/ad-delivery-simulator/pkg/kafka"
	"github.com/ad-delivery-simulator/pkg/redis"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type Engine struct {
	campaignService *campaign.Service
	redis           *redis.Client
	kafka           *kafka.Producer
	brokers         []string
	logger          *logrus.Logger
	auctionTimeout  time.Duration
}

type BidEntry struct {
	Bid        *models.Bid
	Campaign   *models.Campaign
	Score      float64
	IsEligible bool
}

func NewEngine(
	campaignService *campaign.Service,
	redisClient *redis.Client,
	kafkaProducer *kafka.Producer,
	brokers []string,
	logger *logrus.Logger,
) *Engine {
	return &Engine{
		campaignService: campaignService,
		redis:           redisClient,
		kafka:           kafkaProducer,
		brokers:         brokers,
		logger:          logger,
		auctionTimeout:  100 * time.Millisecond,
	}
}

func (e *Engine) RunAuction(ctx context.Context, request *models.BidRequest) (*models.BidResponse, error) {
	startTime := time.Now()
	
	auctionCtx, cancel := context.WithTimeout(ctx, e.auctionTimeout)
	defer cancel()

	e.publishBidRequest(ctx, request)

	activeCampaigns, err := e.campaignService.ListActiveCampaigns(auctionCtx)
	if err != nil {
		e.logger.WithError(err).Error("Failed to get active campaigns")
		return nil, fmt.Errorf("failed to get active campaigns: %w", err)
	}

	if len(activeCampaigns) == 0 {
		return e.createNoBidResponse(request.ID), nil
	}

	bidEntries := e.collectBids(auctionCtx, request, activeCampaigns)
	
	if len(bidEntries) == 0 {
		return e.createNoBidResponse(request.ID), nil
	}

	winner, secondPrice := e.selectWinner(bidEntries)
	
	if winner == nil {
		return e.createNoBidResponse(request.ID), nil
	}

	finalPrice := e.determineFinalPrice(winner.Bid.Price, secondPrice, request.Imp[0].BidFloor)
	
	allowed, err := e.campaignService.CheckAndDecrementBudget(ctx, winner.Campaign.ID, finalPrice)
	if err != nil || !allowed {
		e.logger.WithError(err).WithField("campaign_id", winner.Campaign.ID).Warn("Budget check failed for winner")
		return e.createNoBidResponse(request.ID), nil
	}

	response := e.createBidResponse(request, winner, finalPrice)
	
	e.recordAuctionResult(ctx, request, winner, finalPrice, secondPrice, len(bidEntries), time.Since(startTime))
	
	e.publishBidResponse(ctx, response)

	return response, nil
}

func (e *Engine) collectBids(ctx context.Context, request *models.BidRequest, campaigns []*models.Campaign) []*BidEntry {
	var wg sync.WaitGroup
	bidChan := make(chan *BidEntry, len(campaigns))

	for _, campaign := range campaigns {
		wg.Add(1)
		go func(c *models.Campaign) {
			defer wg.Done()
			
			if entry := e.createBidEntry(ctx, request, c); entry != nil && entry.IsEligible {
				bidChan <- entry
			}
		}(campaign)
	}

	wg.Wait()
	close(bidChan)

	var bidEntries []*BidEntry
	for entry := range bidChan {
		bidEntries = append(bidEntries, entry)
	}

	return bidEntries
}

func (e *Engine) createBidEntry(ctx context.Context, request *models.BidRequest, campaign *models.Campaign) *BidEntry {
	if !e.checkTargeting(request, campaign) {
		return nil
	}

	if request.User.ID != "" {
		allowed, err := e.campaignService.CheckFrequencyCap(ctx, request.User.ID, campaign.ID, "impression")
		if err != nil || !allowed {
			e.logger.WithField("campaign_id", campaign.ID).Debug("Frequency cap exceeded")
			return nil
		}
	}

	pacingRate, _ := e.campaignService.CalculatePacingRate(ctx, campaign.ID)
	if rand.Float64() > pacingRate {
		e.logger.WithField("campaign_id", campaign.ID).Debug("Pacing check failed")
		return nil
	}

	bidAmount := e.calculateBidAmount(campaign, request)
	
	if len(request.Imp) > 0 && bidAmount < request.Imp[0].BidFloor {
		return nil
	}

	bid := &models.Bid{
		ID:    uuid.New().String(),
		ImpID: request.Imp[0].ID,
		Price: bidAmount,
		AdID:  campaign.ID.String(),
		CID:   campaign.ID.String(),
		CrID:  fmt.Sprintf("creative_%s", campaign.ID.String()),
		NURL:  fmt.Sprintf("/track/win?bid=${AUCTION_PRICE}&campaign=%s", campaign.ID),
		IURL:  fmt.Sprintf("/track/impression?campaign=%s", campaign.ID),
		ADomain: []string{"example.com"},
	}

	return &BidEntry{
		Bid:        bid,
		Campaign:   campaign,
		Score:      e.calculateBidScore(campaign, bidAmount, request),
		IsEligible: true,
	}
}

func (e *Engine) checkTargeting(request *models.BidRequest, campaign *models.Campaign) bool {
	if campaign.TargetingRules == nil {
		return true
	}

	rules := campaign.TargetingRules

	if len(rules.GeoTargeting) > 0 && request.Device.Geo != nil {
		if !contains(rules.GeoTargeting, request.Device.Geo.Country) {
			return false
		}
	}

	if len(rules.DeviceTypes) > 0 {
		deviceType := fmt.Sprintf("%d", request.Device.DeviceType)
		if !contains(rules.DeviceTypes, deviceType) {
			return false
		}
	}

	if len(rules.DayParting) > 0 {
		now := time.Now()
		dayOfWeek := int(now.Weekday())
		hour := now.Hour()
		
		isAllowed := false
		for _, rule := range rules.DayParting {
			if rule.DayOfWeek == dayOfWeek && hour >= rule.StartHour && hour < rule.EndHour {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			return false
		}
	}

	return true
}

func (e *Engine) calculateBidAmount(campaign *models.Campaign, request *models.BidRequest) float64 {
	baseBid := campaign.BidAmount
	
	multiplier := 1.0
	
	if request.Device.DeviceType == 1 {
		multiplier *= 1.2
	}
	
	if request.Site != nil && len(request.Site.Cat) > 0 {
		multiplier *= 1.1
	}
	
	return baseBid * multiplier
}

func (e *Engine) calculateBidScore(campaign *models.Campaign, bidAmount float64, request *models.BidRequest) float64 {
	score := bidAmount
	
	if campaign.BidType == models.BidTypeCPC {
		score *= 0.8
	} else if campaign.BidType == models.BidTypeCPA {
		score *= 0.6
	}
	
	remainingBudget := campaign.BudgetDaily - campaign.SpentDaily
	if remainingBudget < campaign.BudgetDaily*0.2 {
		score *= 0.9
	}
	
	return score
}

func (e *Engine) selectWinner(bidEntries []*BidEntry) (*BidEntry, float64) {
	if len(bidEntries) == 0 {
		return nil, 0
	}

	sort.Slice(bidEntries, func(i, j int) bool {
		return bidEntries[i].Score > bidEntries[j].Score
	})

	winner := bidEntries[0]
	
	var secondPrice float64
	if len(bidEntries) > 1 {
		secondPrice = bidEntries[1].Bid.Price
	} else {
		secondPrice = winner.Bid.Price * 0.8
	}

	return winner, secondPrice
}

func (e *Engine) determineFinalPrice(winningBid, secondPrice, bidFloor float64) float64 {
	finalPrice := secondPrice + 0.01
	
	if finalPrice < bidFloor {
		finalPrice = bidFloor
	}
	
	if finalPrice > winningBid {
		finalPrice = winningBid
	}
	
	return finalPrice
}

func (e *Engine) createBidResponse(request *models.BidRequest, winner *BidEntry, finalPrice float64) *models.BidResponse {
	winner.Bid.Price = finalPrice
	
	return &models.BidResponse{
		ID:    request.ID,
		BidID: uuid.New().String(),
		Cur:   "USD",
		SeatBid: []models.SeatBid{
			{
				Bid:  []models.Bid{*winner.Bid},
				Seat: "advertiser-1",
			},
		},
	}
}

func (e *Engine) createNoBidResponse(requestID string) *models.BidResponse {
	return &models.BidResponse{
		ID:      requestID,
		BidID:   uuid.New().String(),
		NBR:     2,
		SeatBid: []models.SeatBid{},
	}
}

func (e *Engine) recordAuctionResult(
	ctx context.Context,
	request *models.BidRequest,
	winner *BidEntry,
	finalPrice, secondPrice float64,
	totalBids int,
	processingTime time.Duration,
) {
	var winningBidID *uuid.UUID
	if winner != nil {
		id := uuid.MustParse(winner.Bid.ID)
		winningBidID = &id
	}

	result := &models.AuctionResult{
		ID:             uuid.New(),
		BidRequestID:   request.ID,
		WinningBidID:   winningBidID,
		WinningPrice:   finalPrice,
		SecondPrice:    secondPrice,
		TotalBids:      totalBids,
		AuctionType:    "second-price",
		ProcessingTime: processingTime.Milliseconds(),
		Timestamp:      time.Now(),
	}

	if err := e.redis.CacheBidRequest(request.ID, result, 5*time.Minute); err != nil {
		e.logger.WithError(err).Error("Failed to cache auction result")
	}

	e.kafka.PublishEvent(ctx, e.brokers, "auction-results", result)
}

func (e *Engine) publishBidRequest(ctx context.Context, request *models.BidRequest) {
	if err := e.kafka.PublishBidRequest(ctx, e.brokers, request); err != nil {
		e.logger.WithError(err).Error("Failed to publish bid request")
	}
}

func (e *Engine) publishBidResponse(ctx context.Context, response *models.BidResponse) {
	if err := e.kafka.PublishBidResponse(ctx, e.brokers, response); err != nil {
		e.logger.WithError(err).Error("Failed to publish bid response")
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}