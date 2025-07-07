package models

import (
	"time"
)

// UserRole represents different user roles in the system
type UserRole string

const (
	RoleUser   UserRole = "user"
	RoleTrader UserRole = "trader"
	RoleAdmin  UserRole = "admin"
	RoleSuper  UserRole = "super_admin"
)

// UserSession represents active user sessions
type UserSession struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"not null;index" json:"user_id"`
	Token        string    `gorm:"unique;not null" json:"-"` // JWT token hash
	RefreshToken string    `gorm:"unique;not null" json:"-"`
	IPAddress    string    `gorm:"not null" json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	ExpiresAt    time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Relationships
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// APIKey represents API keys for programmatic access
type APIKey struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	Name        string    `gorm:"not null" json:"name"`
	KeyID       string    `gorm:"unique;not null;index" json:"key_id"`
	SecretHash  string    `gorm:"not null" json:"-"` // Hashed secret
	Permissions string    `gorm:"type:text" json:"permissions"` // JSON array of permissions
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relationships
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// TwoFactorAuth represents 2FA settings for users
type TwoFactorAuth struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"unique;not null" json:"user_id"`
	Secret       string    `gorm:"not null" json:"-"` // TOTP secret (encrypted)
	BackupCodes  string    `gorm:"type:text" json:"-"` // JSON array of backup codes
	IsEnabled    bool      `gorm:"default:false" json:"is_enabled"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Relationships
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// LoginAttempt represents login attempts for security monitoring
type LoginAttempt struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"not null;index" json:"email"`
	IPAddress string    `gorm:"not null;index" json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Success   bool      `gorm:"not null;index" json:"success"`
	Reason    string    `json:"reason,omitempty"` // Failure reason
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// RateLimit represents rate limiting data
type RateLimit struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"unique;not null;index" json:"key"` // IP or UserID
	Count     int       `gorm:"not null" json:"count"`
	WindowStart time.Time `gorm:"not null;index" json:"window_start"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserPassword represents user password hashes
type UserPassword struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"unique;not null;index" json:"user_id"`
	PasswordHash string    `gorm:"not null" json:"-"` // Hashed password
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Relationships
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// UserRole field needs to be added to the existing User model
// This would be added to pkg/models/user.go:
// Role UserRole `gorm:"not null;default:'user'" json:"role"`

// TableName methods
func (UserSession) TableName() string   { return "user_sessions" }
func (APIKey) TableName() string        { return "api_keys" }
func (TwoFactorAuth) TableName() string { return "two_factor_auth" }
func (LoginAttempt) TableName() string  { return "login_attempts" }
func (RateLimit) TableName() string     { return "rate_limits" }
func (UserPassword) TableName() string  { return "user_passwords" } 