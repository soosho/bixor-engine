package models

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"unique;not null" json:"email"`
	Username  string    `gorm:"unique;not null" json:"username"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	IsVerified bool     `gorm:"default:false" json:"is_verified"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Balances []Balance `gorm:"foreignKey:UserID" json:"balances,omitempty"`
	Orders   []Order   `gorm:"foreignKey:UserID" json:"orders,omitempty"`
}

// Balance represents user's asset balances
type Balance struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	UserID    uint            `gorm:"not null;index" json:"user_id"`
	Asset     string          `gorm:"not null;size:10" json:"asset"`
	Available decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"available"`
	Locked    decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"locked"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`

	// Relationships
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// Market represents a trading pair
type Market struct {
	ID          string          `gorm:"primaryKey" json:"id"`          // e.g., "BTC-USDT"
	BaseAsset   string          `gorm:"not null;size:10" json:"base_asset"`   // e.g., "BTC"
	QuoteAsset  string          `gorm:"not null;size:10" json:"quote_asset"`  // e.g., "USDT"
	IsActive    bool            `gorm:"default:true" json:"is_active"`
	MinSize     decimal.Decimal `gorm:"type:decimal(20,8)" json:"min_size"`
	MaxSize     decimal.Decimal `gorm:"type:decimal(20,8)" json:"max_size"`
	PricePrecision int          `gorm:"default:2" json:"price_precision"`
	SizePrecision  int          `gorm:"default:8" json:"size_precision"`
	TakerFee    decimal.Decimal `gorm:"type:decimal(5,4);default:0.001" json:"taker_fee"`  // 0.1%
	MakerFee    decimal.Decimal `gorm:"type:decimal(5,4);default:0.001" json:"maker_fee"`  // 0.1%
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`

	// Relationships
	Orders []Order `gorm:"foreignKey:MarketID" json:"orders,omitempty"`
	Trades []Trade `gorm:"foreignKey:MarketID" json:"trades,omitempty"`
}

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
	ID           string          `gorm:"primaryKey" json:"id"`
	UserID       uint            `gorm:"not null;index" json:"user_id"`
	MarketID     string          `gorm:"not null;index" json:"market_id"`
	Side         OrderSide       `gorm:"not null" json:"side"`
	Type         OrderType       `gorm:"not null" json:"type"`
	Status       OrderStatus     `gorm:"not null;default:'pending'" json:"status"`
	Price        decimal.Decimal `gorm:"type:decimal(20,8)" json:"price"`
	Size         decimal.Decimal `gorm:"type:decimal(20,8);not null" json:"size"`
	FilledSize   decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"filled_size"`
	RemainingSize decimal.Decimal `gorm:"type:decimal(20,8)" json:"remaining_size"`
	Fee          decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"fee"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	FilledAt     *time.Time      `json:"filled_at,omitempty"`
	CancelledAt  *time.Time      `json:"cancelled_at,omitempty"`

	// Relationships
	User   User    `gorm:"foreignKey:UserID" json:"-"`
	Market Market  `gorm:"foreignKey:MarketID" json:"-"`
	Trades []Trade `gorm:"foreignKey:TakerOrderID;foreignKey:MakerOrderID" json:"trades,omitempty"`
}

// Trade represents a completed trade
type Trade struct {
	ID             uint            `gorm:"primaryKey" json:"id"`
	MarketID       string          `gorm:"not null;index" json:"market_id"`
	TakerOrderID   string          `gorm:"not null;index" json:"taker_order_id"`
	MakerOrderID   string          `gorm:"not null;index" json:"maker_order_id"`
	TakerUserID    uint            `gorm:"not null;index" json:"taker_user_id"`
	MakerUserID    uint            `gorm:"not null;index" json:"maker_user_id"`
	Price          decimal.Decimal `gorm:"type:decimal(20,8);not null" json:"price"`
	Size           decimal.Decimal `gorm:"type:decimal(20,8);not null" json:"size"`
	TakerSide      OrderSide       `gorm:"not null" json:"taker_side"`
	TakerFee       decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"taker_fee"`
	MakerFee       decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"maker_fee"`
	CreatedAt      time.Time       `gorm:"index" json:"created_at"`

	// Relationships
	Market     Market `gorm:"foreignKey:MarketID" json:"-"`
	TakerOrder Order  `gorm:"foreignKey:TakerOrderID" json:"-"`
	MakerOrder Order  `gorm:"foreignKey:MakerOrderID" json:"-"`
	TakerUser  User   `gorm:"foreignKey:TakerUserID" json:"-"`
	MakerUser  User   `gorm:"foreignKey:MakerUserID" json:"-"`
}

// MarketData represents market data for a trading pair
type MarketData struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	MarketID    string          `gorm:"not null;index" json:"market_id"`
	Price       decimal.Decimal `gorm:"type:decimal(20,8);not null" json:"price"`
	Volume24h   decimal.Decimal `gorm:"type:decimal(20,8);default:0" json:"volume_24h"`
	High24h     decimal.Decimal `gorm:"type:decimal(20,8)" json:"high_24h"`
	Low24h      decimal.Decimal `gorm:"type:decimal(20,8)" json:"low_24h"`
	Change24h   decimal.Decimal `gorm:"type:decimal(10,4)" json:"change_24h"`
	BestBid     decimal.Decimal `gorm:"type:decimal(20,8)" json:"best_bid"`
	BestAsk     decimal.Decimal `gorm:"type:decimal(20,8)" json:"best_ask"`
	Spread      decimal.Decimal `gorm:"type:decimal(20,8)" json:"spread"`
	UpdatedAt   time.Time       `json:"updated_at"`

	// Relationships
	Market Market `gorm:"foreignKey:MarketID" json:"-"`
}

// BeforeCreate hooks
func (o *Order) BeforeCreate(tx *gorm.DB) error {
	if o.RemainingSize.IsZero() {
		o.RemainingSize = o.Size
	}
	return nil
}

func (b *Balance) BeforeCreate(tx *gorm.DB) error {
	if b.Available.IsZero() {
		b.Available = decimal.Zero
	}
	if b.Locked.IsZero() {
		b.Locked = decimal.Zero
	}
	return nil
}

// TableName methods for custom table names
func (User) TableName() string       { return "users" }
func (Balance) TableName() string    { return "balances" }
func (Market) TableName() string     { return "markets" }
func (Order) TableName() string      { return "orders" }
func (Trade) TableName() string      { return "trades" }
func (MarketData) TableName() string { return "market_data" } 