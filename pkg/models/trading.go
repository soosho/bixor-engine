package models

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// OrderStatus represents the status of an order
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusOpen      OrderStatus = "open"
	OrderStatusFilled    OrderStatus = "filled"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusExpired   OrderStatus = "expired"
)

// OrderType represents the type of an order
type OrderType string

const (
	OrderTypeMarket   OrderType = "market"
	OrderTypeLimit    OrderType = "limit"
	OrderTypeIOC      OrderType = "ioc"
	OrderTypeFOK      OrderType = "fok"
	OrderTypePostOnly OrderType = "post_only"
)

// OrderSide represents the side of an order
type OrderSide int8

const (
	OrderSideBuy  OrderSide = 1
	OrderSideSell OrderSide = 2
)

// Order represents a trading order
type Order struct {
	ID            string          `gorm:"primaryKey" json:"id"`
	UserID        uint            `gorm:"not null;index" json:"user_id"`
	MarketID      string          `gorm:"not null;index" json:"market_id"`
	Side          OrderSide       `gorm:"not null" json:"side"`
	Type          OrderType       `gorm:"not null" json:"type"`
	Status        OrderStatus     `gorm:"not null;default:'pending'" json:"status"`
	Price         decimal.Decimal `gorm:"type:decimal(20,8)" json:"price"`
	Size          decimal.Decimal `gorm:"type:decimal(20,8);not null" json:"size"`
	FilledSize    decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"filled_size"`
	RemainingSize decimal.Decimal `gorm:"type:decimal(20,8)" json:"remaining_size"`
	Fee           decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"fee"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	FilledAt      *time.Time      `json:"filled_at,omitempty"`
	CancelledAt   *time.Time      `json:"cancelled_at,omitempty"`

	// Relationships
	User   User    `gorm:"foreignKey:UserID" json:"-"`
	Market Market  `gorm:"foreignKey:MarketID" json:"-"`
	Trades []Trade `gorm:"foreignKey:TakerOrderID;foreignKey:MakerOrderID" json:"trades,omitempty"`
}

// Trade represents a completed trade
type Trade struct {
	ID           uint            `gorm:"primaryKey" json:"id"`
	MarketID     string          `gorm:"not null;index" json:"market_id"`
	TakerOrderID string          `gorm:"not null;index" json:"taker_order_id"`
	MakerOrderID string          `gorm:"not null;index" json:"maker_order_id"`
	TakerUserID  uint            `gorm:"not null;index" json:"taker_user_id"`
	MakerUserID  uint            `gorm:"not null;index" json:"maker_user_id"`
	Price        decimal.Decimal `gorm:"type:decimal(20,8);not null" json:"price"`
	Size         decimal.Decimal `gorm:"type:decimal(20,8);not null" json:"size"`
	TakerSide    OrderSide       `gorm:"not null" json:"taker_side"`
	TakerFee     decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"taker_fee"`
	MakerFee     decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"maker_fee"`
	CreatedAt    time.Time       `gorm:"index" json:"created_at"`

	// Relationships
	Market     Market `gorm:"foreignKey:MarketID" json:"-"`
	TakerOrder Order  `gorm:"foreignKey:TakerOrderID" json:"-"`
	MakerOrder Order  `gorm:"foreignKey:MakerOrderID" json:"-"`
	TakerUser  User   `gorm:"foreignKey:TakerUserID" json:"-"`
	MakerUser  User   `gorm:"foreignKey:MakerUserID" json:"-"`
}

// BeforeCreate hook for Order
func (o *Order) BeforeCreate(tx *gorm.DB) error {
	if o.RemainingSize.IsZero() {
		o.RemainingSize = o.Size
	}
	return nil
}

// TableName methods
func (Order) TableName() string { return "orders" }
func (Trade) TableName() string { return "trades" } 