package api

import (
	"net/http"
	"time"

	"github.com/ad-delivery-simulator/internal/auction"
	"github.com/ad-delivery-simulator/internal/campaign"
	"github.com/ad-delivery-simulator/internal/models"
	"github.com/ad-delivery-simulator/internal/tracking"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type Handlers struct {
	auctionEngine   *auction.Engine
	campaignService *campaign.Service
	trackingService *tracking.Service
	logger          *logrus.Logger
}

func NewHandlers(
	auctionEngine *auction.Engine,
	campaignService *campaign.Service,
	trackingService *tracking.Service,
	logger *logrus.Logger,
) *Handlers {
	return &Handlers{
		auctionEngine:   auctionEngine,
		campaignService: campaignService,
		trackingService: trackingService,
		logger:          logger,
	}
}

func (h *Handlers) HandleBidRequest(c *gin.Context) {
	var request models.BidRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.WithError(err).Error("Failed to parse bid request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid bid request format"})
		return
	}

	if request.ID == "" {
		request.ID = uuid.New().String()
	}

	response, err := h.auctionEngine.RunAuction(c.Request.Context(), &request)
	if err != nil {
		h.logger.WithError(err).Error("Failed to run auction")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Auction failed"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handlers) CreateCampaign(c *gin.Context) {
	var campaign models.Campaign
	if err := c.ShouldBindJSON(&campaign); err != nil {
		h.logger.WithError(err).Error("Failed to parse campaign request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign format"})
		return
	}

	if err := h.campaignService.CreateCampaign(c.Request.Context(), &campaign); err != nil {
		h.logger.WithError(err).Error("Failed to create campaign")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create campaign"})
		return
	}

	c.JSON(http.StatusCreated, campaign)
}

func (h *Handlers) GetCampaign(c *gin.Context) {
	campaignID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	campaign, err := h.campaignService.GetCampaign(c.Request.Context(), campaignID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get campaign")
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	c.JSON(http.StatusOK, campaign)
}

func (h *Handlers) UpdateCampaign(c *gin.Context) {
	campaignID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	var campaign models.Campaign
	if err := c.ShouldBindJSON(&campaign); err != nil {
		h.logger.WithError(err).Error("Failed to parse campaign request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign format"})
		return
	}

	campaign.ID = campaignID

	if err := h.campaignService.UpdateCampaign(c.Request.Context(), &campaign); err != nil {
		h.logger.WithError(err).Error("Failed to update campaign")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update campaign"})
		return
	}

	c.JSON(http.StatusOK, campaign)
}

func (h *Handlers) ListCampaigns(c *gin.Context) {
	campaigns, err := h.campaignService.ListActiveCampaigns(c.Request.Context())
	if err != nil {
		h.logger.WithError(err).Error("Failed to list campaigns")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list campaigns"})
		return
	}

	c.JSON(http.StatusOK, campaigns)
}

func (h *Handlers) GetCampaignPerformance(c *gin.Context) {
	campaignID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	date := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
	
	metrics, err := h.campaignService.GetCampaignMetrics(c.Request.Context(), campaignID, date)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get campaign metrics")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get metrics"})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

func (h *Handlers) TrackImpression(c *gin.Context) {
	var request struct {
		CampaignID string `json:"campaign_id" binding:"required"`
		CreativeID string `json:"creative_id"`
		UserID     string `json:"user_id"`
		SessionID  string `json:"session_id"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	campaignID, err := uuid.Parse(request.CampaignID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	var creativeID uuid.UUID
	if request.CreativeID != "" {
		creativeID, _ = uuid.Parse(request.CreativeID)
	}

	event := &models.TrackingEvent{
		CampaignID: campaignID,
		CreativeID: creativeID,
		UserID:     request.UserID,
		SessionID:  request.SessionID,
		IP:         c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		Referrer:   c.Request.Referer(),
	}

	if err := h.trackingService.TrackImpression(c.Request.Context(), event); err != nil {
		h.logger.WithError(err).Error("Failed to track impression")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to track impression"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "event_id": event.ID})
}

func (h *Handlers) TrackClick(c *gin.Context) {
	var request struct {
		CampaignID string `json:"campaign_id" binding:"required"`
		CreativeID string `json:"creative_id"`
		UserID     string `json:"user_id"`
		SessionID  string `json:"session_id"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	campaignID, err := uuid.Parse(request.CampaignID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	var creativeID uuid.UUID
	if request.CreativeID != "" {
		creativeID, _ = uuid.Parse(request.CreativeID)
	}

	event := &models.TrackingEvent{
		CampaignID: campaignID,
		CreativeID: creativeID,
		UserID:     request.UserID,
		SessionID:  request.SessionID,
		IP:         c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		Referrer:   c.Request.Referer(),
	}

	if err := h.trackingService.TrackClick(c.Request.Context(), event); err != nil {
		h.logger.WithError(err).Error("Failed to track click")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to track click"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "event_id": event.ID})
}

func (h *Handlers) TrackConversion(c *gin.Context) {
	var request struct {
		CampaignID string  `json:"campaign_id" binding:"required"`
		UserID     string  `json:"user_id"`
		Value      float64 `json:"value"`
		SessionID  string  `json:"session_id"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	campaignID, err := uuid.Parse(request.CampaignID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	event := &models.TrackingEvent{
		CampaignID: campaignID,
		UserID:     request.UserID,
		SessionID:  request.SessionID,
		IP:         c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		Price:      request.Value,
	}

	if err := h.trackingService.TrackConversion(c.Request.Context(), event); err != nil {
		h.logger.WithError(err).Error("Failed to track conversion")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to track conversion"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "event_id": event.ID})
}

func (h *Handlers) GetEventStats(c *gin.Context) {
	campaignID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	startTimeStr := c.DefaultQuery("start", time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	endTimeStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start time format"})
		return
	}

	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end time format"})
		return
	}

	stats, err := h.trackingService.GetEventStats(c.Request.Context(), campaignID, startTime, endTime)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get event stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *Handlers) GetRealTimeMetrics(c *gin.Context) {
	campaignID := c.Param("id")
	
	metrics, err := h.trackingService.GetRealTimeMetrics(c.Request.Context(), campaignID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get real-time metrics")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get metrics"})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

func (h *Handlers) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"time":   time.Now().Unix(),
	})
}

func (h *Handlers) GetMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Prometheus metrics available at /metrics",
	})
}