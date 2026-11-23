package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type Item struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	URL           string          `json:"url"`
	NormalizedURL string          `json:"normalized_url"`
	Store         string          `json:"store"`
	Category      string          `json:"category"`
	GTIN          *string         `json:"gtin,omitempty"`
	Brand         *string         `json:"brand,omitempty"`
	TargetPrice   decimal.Decimal `json:"target_price"`
	Currency      string          `json:"currency"`
	Active        bool            `json:"active"`
	LastFetchedAt *time.Time      `json:"last_fetched_at,omitempty"`
	LastError     *string         `json:"last_error,omitempty"`
	FailCount     int             `json:"fail_count"`
	CreatedAt     time.Time       `json:"created_at"`
}

type PriceSource string

const (
	SourceJSONLD PriceSource = "jsonld"
	SourceOG     PriceSource = "og"
	SourceManual PriceSource = "manual"
)

type PricePoint struct {
	ID        string          `json:"id"`
	ItemID    string          `json:"item_id"`
	Price     decimal.Decimal `json:"price"`
	Currency  string          `json:"currency"`
	InStock   *bool           `json:"in_stock,omitempty"`
	Source    PriceSource     `json:"source"`
	FetchedAt time.Time       `json:"fetched_at"`
}

type NotificationType string

const (
	NotifyTargetReached NotificationType = "target_reached"
	NotifyNewLow        NotificationType = "new_low"
)

type Notification struct {
	ID     string           `json:"id"`
	ItemID string           `json:"item_id"`
	Type   NotificationType `json:"type"`
	Price  decimal.Decimal  `json:"price"`
	SentAt time.Time        `json:"sent_at"`
}

type PushSubscription struct {
	ID        string    `json:"id"`
	Endpoint  string    `json:"endpoint"`
	P256dh    string    `json:"p256dh"`
	Auth      string    `json:"auth"`
	CreatedAt time.Time `json:"created_at"`
}
