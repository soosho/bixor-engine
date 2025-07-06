package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// Market represents a trading pair
type Market struct {
	ID             string          `gorm:"primaryKey" json:"id"`                    // e.g., "BTC-USDT"
	BaseAsset      string          `gorm:"not null;size:10" json:"base_asset"`      // e.g., "BTC"
	QuoteAsset     string          `gorm:"not null;size:10" json:"quote_asset"`     // e.g., "USDT"
	IsActive       bool            `gorm:"default:true" json:"is_active"`
	MinSize        decimal.Decimal `gorm:"type:decimal(20,8)" json:"min_size"`
	MaxSize        decimal.Decimal `gorm:"type:decimal(20,8)" json:"max_size"`
	PricePrecision int             `gorm:"default:2" json:"price_precision"`
	SizePrecision  int             `gorm:"default:8" json:"size_precision"`
	TakerFee       decimal.Decimal `gorm:"type:decimal(5,4);default:0.001" json:"taker_fee"` // 0.1%
	MakerFee       decimal.Decimal `gorm:"type:decimal(5,4);default:0.001" json:"maker_fee"` // 0.1%
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`

	// Relationships
	Orders []Order `gorm:"foreignKey:MarketID" json:"orders,omitempty"`
	Trades []Trade `gorm:"foreignKey:MarketID" json:"trades,omitempty"`
}

// MarketData represents market data for a trading pair
type MarketData struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	MarketID  string          `gorm:"not null;index" json:"market_id"`
	Price     decimal.Decimal `gorm:"type:decimal(20,8);not null" json:"price"`
	Volume24h decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"volume_24h"`
	High24h   decimal.Decimal `gorm:"type:decimal(20,8)" json:"high_24h"`
	Low24h    decimal.Decimal `gorm:"type:decimal(20,8)" json:"low_24h"`
	Change24h decimal.Decimal `gorm:"type:decimal(10,4)" json:"change_24h"`
	BestBid   decimal.Decimal `gorm:"type:decimal(20,8)" json:"best_bid"`
	BestAsk   decimal.Decimal `gorm:"type:decimal(20,8)" json:"best_ask"`
	Spread    decimal.Decimal `gorm:"type:decimal(20,8)" json:"spread"`
	UpdatedAt time.Time       `json:"updated_at"`

	// Relationships
	Market Market `gorm:"foreignKey:MarketID" json:"-"`
}

// TableName methods
func (Market) TableName() string     { return "markets" }
func (MarketData) TableName() string { return "market_data" } 