package models

import (
	"time"

	"github.com/google/uuid"
)

type AdCreative struct {
	ID           uuid.UUID      `json:"id" db:"id"`
	CampaignID   uuid.UUID      `json:"campaign_id" db:"campaign_id"`
	Name         string         `json:"name" db:"name"`
	Type         CreativeType   `json:"type" db:"type"`
	Format       CreativeFormat `json:"format" db:"format"`
	Width        int            `json:"width" db:"width"`
	Height       int            `json:"height" db:"height"`
	AssetURL     string         `json:"asset_url" db:"asset_url"`
	ClickURL     string         `json:"click_url" db:"click_url"`
	ImpressionURL string        `json:"impression_url" db:"impression_url"`
	HTML         string         `json:"html" db:"html"`
	Status       string         `json:"status" db:"status"`
	CreatedAt    time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at" db:"updated_at"`
}

type CreativeType string

const (
	CreativeTypeBanner CreativeType = "banner"
	CreativeTypeVideo  CreativeType = "video"
	CreativeTypeNative CreativeType = "native"
	CreativeTypeAudio  CreativeType = "audio"
)

type CreativeFormat string

const (
	CreativeFormat300x250   CreativeFormat = "300x250"
	CreativeFormat728x90    CreativeFormat = "728x90"
	CreativeFormat320x50    CreativeFormat = "320x50"
	CreativeFormat160x600   CreativeFormat = "160x600"
	CreativeFormat970x250   CreativeFormat = "970x250"
	CreativeFormat300x600   CreativeFormat = "300x600"
	CreativeFormatResponsive CreativeFormat = "responsive"
)

type TrackingEvent struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Type         EventType  `json:"type" db:"type"`
	CampaignID   uuid.UUID  `json:"campaign_id" db:"campaign_id"`
	CreativeID   uuid.UUID  `json:"creative_id" db:"creative_id"`
	UserID       string     `json:"user_id" db:"user_id"`
	SessionID    string     `json:"session_id" db:"session_id"`
	IP           string     `json:"ip" db:"ip"`
	UserAgent    string     `json:"user_agent" db:"user_agent"`
	Referrer     string     `json:"referrer" db:"referrer"`
	Price        float64    `json:"price" db:"price"`
	Timestamp    time.Time  `json:"timestamp" db:"timestamp"`
	ProcessedAt  *time.Time `json:"processed_at" db:"processed_at"`
	Metadata     string     `json:"metadata" db:"metadata"`
}

type EventType string

const (
	EventTypeImpression EventType = "impression"
	EventTypeClick      EventType = "click"
	EventTypeConversion EventType = "conversion"
	EventTypeViewable   EventType = "viewable"
)

type AuctionResult struct {
	ID              uuid.UUID  `json:"id"`
	BidRequestID    string     `json:"bid_request_id"`
	WinningBidID    *uuid.UUID `json:"winning_bid_id"`
	WinningPrice    float64    `json:"winning_price"`
	SecondPrice     float64    `json:"second_price"`
	TotalBids       int        `json:"total_bids"`
	AuctionType     string     `json:"auction_type"`
	ProcessingTime  int64      `json:"processing_time_ms"`
	Timestamp       time.Time  `json:"timestamp"`
}