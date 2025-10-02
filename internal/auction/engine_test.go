package auction

import (
	"context"
	"testing"
	"time"

	"github.com/ad-delivery-simulator/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockCampaignService struct {
	mock.Mock
}

func (m *MockCampaignService) ListActiveCampaigns(ctx context.Context) ([]*models.Campaign, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*models.Campaign), args.Error(1)
}

func (m *MockCampaignService) CheckFrequencyCap(ctx context.Context, userID string, campaignID uuid.UUID, eventType string) (bool, error) {
	args := m.Called(ctx, userID, campaignID, eventType)
	return args.Bool(0), args.Error(1)
}

func (m *MockCampaignService) CalculatePacingRate(ctx context.Context, campaignID uuid.UUID) (float64, error) {
	args := m.Called(ctx, campaignID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockCampaignService) CheckAndDecrementBudget(ctx context.Context, campaignID uuid.UUID, amount float64) (bool, error) {
	args := m.Called(ctx, campaignID, amount)
	return args.Bool(0), args.Error(1)
}

func TestEngine_SelectWinner(t *testing.T) {
	engine := &Engine{}

	tests := []struct {
		name           string
		bidEntries     []*BidEntry
		expectedWinner *BidEntry
		expectedSecond float64
	}{
		{
			name:           "No bids",
			bidEntries:     []*BidEntry{},
			expectedWinner: nil,
			expectedSecond: 0,
		},
		{
			name: "Single bid",
			bidEntries: []*BidEntry{
				{
					Bid:   &models.Bid{Price: 1.50},
					Score: 1.50,
				},
			},
			expectedWinner: &BidEntry{
				Bid:   &models.Bid{Price: 1.50},
				Score: 1.50,
			},
			expectedSecond: 1.20,
		},
		{
			name: "Multiple bids",
			bidEntries: []*BidEntry{
				{
					Bid:   &models.Bid{Price: 2.00},
					Score: 2.00,
				},
				{
					Bid:   &models.Bid{Price: 1.50},
					Score: 1.50,
				},
				{
					Bid:   &models.Bid{Price: 1.00},
					Score: 1.00,
				},
			},
			expectedWinner: &BidEntry{
				Bid:   &models.Bid{Price: 2.00},
				Score: 2.00,
			},
			expectedSecond: 1.50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			winner, secondPrice := engine.selectWinner(tt.bidEntries)
			
			if tt.expectedWinner == nil {
				assert.Nil(t, winner)
			} else {
				assert.NotNil(t, winner)
				assert.Equal(t, tt.expectedWinner.Score, winner.Score)
			}
			
			assert.Equal(t, tt.expectedSecond, secondPrice)
		})
	}
}

func TestEngine_DetermineFinalPrice(t *testing.T) {
	engine := &Engine{}

	tests := []struct {
		name         string
		winningBid   float64
		secondPrice  float64
		bidFloor     float64
		expectedPrice float64
	}{
		{
			name:         "Second price plus penny",
			winningBid:   2.00,
			secondPrice:  1.50,
			bidFloor:     1.00,
			expectedPrice: 1.51,
		},
		{
			name:         "Floor price when second price too low",
			winningBid:   2.00,
			secondPrice:  0.50,
			bidFloor:     1.00,
			expectedPrice: 1.00,
		},
		{
			name:         "Winning bid when second price too high",
			winningBid:   1.50,
			secondPrice:  1.60,
			bidFloor:     1.00,
			expectedPrice: 1.50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price := engine.determineFinalPrice(tt.winningBid, tt.secondPrice, tt.bidFloor)
			assert.Equal(t, tt.expectedPrice, price)
		})
	}
}

func TestEngine_CheckTargeting(t *testing.T) {
	engine := &Engine{}

	tests := []struct {
		name      string
		request   *models.BidRequest
		campaign  *models.Campaign
		expected  bool
	}{
		{
			name: "No targeting rules",
			request: &models.BidRequest{
				Device: models.Device{
					DeviceType: 1,
				},
			},
			campaign: &models.Campaign{
				TargetingRules: nil,
			},
			expected: true,
		},
		{
			name: "Geo targeting match",
			request: &models.BidRequest{
				Device: models.Device{
					Geo: &models.Geo{
						Country: "US",
					},
				},
			},
			campaign: &models.Campaign{
				TargetingRules: &models.TargetingRules{
					GeoTargeting: []string{"US", "CA"},
				},
			},
			expected: true,
		},
		{
			name: "Geo targeting no match",
			request: &models.BidRequest{
				Device: models.Device{
					Geo: &models.Geo{
						Country: "UK",
					},
				},
			},
			campaign: &models.Campaign{
				TargetingRules: &models.TargetingRules{
					GeoTargeting: []string{"US", "CA"},
				},
			},
			expected: false,
		},
		{
			name: "Device type match",
			request: &models.BidRequest{
				Device: models.Device{
					DeviceType: 1,
				},
			},
			campaign: &models.Campaign{
				TargetingRules: &models.TargetingRules{
					DeviceTypes: []string{"1", "2"},
				},
			},
			expected: true,
		},
		{
			name: "Day parting match",
			request: &models.BidRequest{},
			campaign: &models.Campaign{
				TargetingRules: &models.TargetingRules{
					DayParting: []models.DayPartRule{
						{
							DayOfWeek: int(time.Now().Weekday()),
							StartHour: 0,
							EndHour:   23,
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.checkTargeting(tt.request, tt.campaign)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEngine_CalculateBidAmount(t *testing.T) {
	engine := &Engine{}

	tests := []struct {
		name     string
		campaign *models.Campaign
		request  *models.BidRequest
		expected float64
	}{
		{
			name: "Base bid only",
			campaign: &models.Campaign{
				BidAmount: 1.00,
			},
			request: &models.BidRequest{
				Device: models.Device{
					DeviceType: 2,
				},
			},
			expected: 1.00,
		},
		{
			name: "Mobile device multiplier",
			campaign: &models.Campaign{
				BidAmount: 1.00,
			},
			request: &models.BidRequest{
				Device: models.Device{
					DeviceType: 1,
				},
			},
			expected: 1.20,
		},
		{
			name: "Site category multiplier",
			campaign: &models.Campaign{
				BidAmount: 1.00,
			},
			request: &models.BidRequest{
				Device: models.Device{
					DeviceType: 2,
				},
				Site: &models.Site{
					Cat: []string{"IAB1"},
				},
			},
			expected: 1.10,
		},
		{
			name: "Combined multipliers",
			campaign: &models.Campaign{
				BidAmount: 1.00,
			},
			request: &models.BidRequest{
				Device: models.Device{
					DeviceType: 1,
				},
				Site: &models.Site{
					Cat: []string{"IAB1"},
				},
			},
			expected: 1.32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amount := engine.calculateBidAmount(tt.campaign, tt.request)
			assert.InDelta(t, tt.expected, amount, 0.01)
		})
	}
}