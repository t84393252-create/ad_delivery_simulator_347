package models

import (
	"time"

	"github.com/google/uuid"
)

type CampaignStatus string

const (
	CampaignStatusActive   CampaignStatus = "active"
	CampaignStatusPaused   CampaignStatus = "paused"
	CampaignStatusDraft    CampaignStatus = "draft"
	CampaignStatusComplete CampaignStatus = "complete"
)

type BidType string

const (
	BidTypeCPM BidType = "CPM"
	BidTypeCPC BidType = "CPC"
	BidTypeCPA BidType = "CPA"
)

type Campaign struct {
	ID               uuid.UUID         `json:"id" db:"id"`
	Name             string            `json:"name" db:"name"`
	AdvertiserID     string            `json:"advertiser_id" db:"advertiser_id"`
	Status           CampaignStatus    `json:"status" db:"status"`
	BudgetDaily      float64           `json:"budget_daily" db:"budget_daily"`
	BudgetTotal      float64           `json:"budget_total" db:"budget_total"`
	SpentDaily       float64           `json:"spent_daily" db:"spent_daily"`
	SpentTotal       float64           `json:"spent_total" db:"spent_total"`
	BidType          BidType           `json:"bid_type" db:"bid_type"`
	BidAmount        float64           `json:"bid_amount" db:"bid_amount"`
	TargetingRules   *TargetingRules   `json:"targeting_rules" db:"targeting_rules"`
	FrequencyCapping *FrequencyCapping `json:"frequency_capping" db:"frequency_capping"`
	StartDate        time.Time         `json:"start_date" db:"start_date"`
	EndDate          *time.Time        `json:"end_date" db:"end_date"`
	CreatedAt        time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at" db:"updated_at"`
}

type TargetingRules struct {
	GeoTargeting    []string          `json:"geo_targeting"`
	DeviceTypes     []string          `json:"device_types"`
	UserSegments    []string          `json:"user_segments"`
	DayParting      []DayPartRule     `json:"day_parting"`
	CustomTargeting map[string]string `json:"custom_targeting"`
}

type DayPartRule struct {
	DayOfWeek int `json:"day_of_week"`
	StartHour int `json:"start_hour"`
	EndHour   int `json:"end_hour"`
}

type FrequencyCapping struct {
	ImpressionCap int           `json:"impression_cap"`
	ClickCap      int           `json:"click_cap"`
	TimeWindow    time.Duration `json:"time_window"`
}

type CampaignMetrics struct {
	CampaignID  uuid.UUID `json:"campaign_id"`
	Impressions int64     `json:"impressions"`
	Clicks      int64     `json:"clicks"`
	Conversions int64     `json:"conversions"`
	Spend       float64   `json:"spend"`
	CTR         float64   `json:"ctr"`
	CPC         float64   `json:"cpc"`
	CPM         float64   `json:"cpm"`
	Date        time.Time `json:"date"`
}