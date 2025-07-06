package models

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Email      string    `gorm:"unique;not null" json:"email"`
	Username   string    `gorm:"unique;not null" json:"username"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	IsActive   bool      `gorm:"default:true" json:"is_active"`
	IsVerified bool      `gorm:"default:false" json:"is_verified"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

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

// BeforeCreate hook for Balance
func (b *Balance) BeforeCreate(tx *gorm.DB) error {
	if b.Available.IsZero() {
		b.Available = decimal.Zero
	}
	if b.Locked.IsZero() {
		b.Locked = decimal.Zero
	}
	return nil
}

// TableName methods
func (User) TableName() string    { return "users" }
func (Balance) TableName() string { return "balances" } 