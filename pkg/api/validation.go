package api

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// Validation patterns
var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,50}$`)
	marketIDRegex = regexp.MustCompile(`^[A-Z]{3,10}-[A-Z]{3,10}$`)
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}
	
	var messages []string
	for _, err := range ve {
		messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	return strings.Join(messages, "; ")
}

// Validator provides validation methods
type Validator struct {
	errors ValidationErrors
}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{
		errors: make(ValidationErrors, 0),
	}
}

// AddError adds a validation error
func (v *Validator) AddError(field, message string) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// HasErrors returns true if there are validation errors
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// GetErrors returns all validation errors
func (v *Validator) GetErrors() ValidationErrors {
	return v.errors
}

// ValidateEmail validates an email address
func (v *Validator) ValidateEmail(field, email string) {
	if email == "" {
		v.AddError(field, "email is required")
		return
	}
	
	if len(email) > 254 {
		v.AddError(field, "email is too long")
		return
	}
	
	if !emailRegex.MatchString(email) {
		v.AddError(field, "invalid email format")
	}
}

// ValidateUsername validates a username
func (v *Validator) ValidateUsername(field, username string) {
	if username == "" {
		v.AddError(field, "username is required")
		return
	}
	
	if len(username) < 3 {
		v.AddError(field, "username must be at least 3 characters")
		return
	}
	
	if len(username) > 50 {
		v.AddError(field, "username must be at most 50 characters")
		return
	}
	
	if !usernameRegex.MatchString(username) {
		v.AddError(field, "username can only contain letters, numbers, underscores, and hyphens")
	}
}

// ValidatePassword validates a password
func (v *Validator) ValidatePassword(field, password string) {
	if password == "" {
		v.AddError(field, "password is required")
		return
	}
	
	if len(password) < 8 {
		v.AddError(field, "password must be at least 8 characters")
		return
	}
	
	if len(password) > 128 {
		v.AddError(field, "password is too long")
		return
	}
	
	// Check for strong password requirements
	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)
	
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}
	
	if !hasUpper {
		v.AddError(field, "password must contain at least one uppercase letter")
	}
	if !hasLower {
		v.AddError(field, "password must contain at least one lowercase letter")
	}
	if !hasNumber {
		v.AddError(field, "password must contain at least one number")
	}
	if !hasSpecial {
		v.AddError(field, "password must contain at least one special character")
	}
}

// ValidateMarketID validates a market ID
func (v *Validator) ValidateMarketID(field, marketID string) {
	if marketID == "" {
		v.AddError(field, "market ID is required")
		return
	}
	
	if !marketIDRegex.MatchString(marketID) {
		v.AddError(field, "invalid market ID format (expected: BASE-QUOTE)")
	}
}

// ValidateOrderSide validates an order side
func (v *Validator) ValidateOrderSide(field string, side int8) {
	if side != 1 && side != 2 {
		v.AddError(field, "invalid order side (1=buy, 2=sell)")
	}
}

// ValidateOrderType validates an order type
func (v *Validator) ValidateOrderType(field, orderType string) {
	validTypes := []string{"market", "limit", "stop", "stop_limit", "fok", "ioc", "post_only"}
	
	if orderType == "" {
		v.AddError(field, "order type is required")
		return
	}
	
	for _, validType := range validTypes {
		if orderType == validType {
			return
		}
	}
	
	v.AddError(field, fmt.Sprintf("invalid order type (valid types: %s)", strings.Join(validTypes, ", ")))
}

// ValidatePrice validates a price
func (v *Validator) ValidatePrice(field, priceStr string, required bool) decimal.Decimal {
	if priceStr == "" {
		if required {
			v.AddError(field, "price is required")
		}
		return decimal.Zero
	}
	
	price, err := decimal.NewFromString(priceStr)
	if err != nil {
		v.AddError(field, "invalid price format")
		return decimal.Zero
	}
	
	if price.IsNegative() {
		v.AddError(field, "price cannot be negative")
		return decimal.Zero
	}
	
	if price.GreaterThan(decimal.NewFromFloat(1000000)) {
		v.AddError(field, "price is too large")
		return decimal.Zero
	}
	
	return price
}

// ValidateSize validates an order size
func (v *Validator) ValidateSize(field, sizeStr string) decimal.Decimal {
	if sizeStr == "" {
		v.AddError(field, "size is required")
		return decimal.Zero
	}
	
	size, err := decimal.NewFromString(sizeStr)
	if err != nil {
		v.AddError(field, "invalid size format")
		return decimal.Zero
	}
	
	if size.IsZero() || size.IsNegative() {
		v.AddError(field, "size must be positive")
		return decimal.Zero
	}
	
	if size.GreaterThan(decimal.NewFromFloat(1000000)) {
		v.AddError(field, "size is too large")
		return decimal.Zero
	}
	
	return size
}

// ValidateString validates a general string field
func (v *Validator) ValidateString(field, value string, minLen, maxLen int, required bool) {
	if value == "" {
		if required {
			v.AddError(field, fmt.Sprintf("%s is required", field))
		}
		return
	}
	
	if len(value) < minLen {
		v.AddError(field, fmt.Sprintf("%s must be at least %d characters", field, minLen))
	}
	
	if maxLen > 0 && len(value) > maxLen {
		v.AddError(field, fmt.Sprintf("%s must be at most %d characters", field, maxLen))
	}
}

// ValidateTOTPCode validates a TOTP code
func (v *Validator) ValidateTOTPCode(field, code string) {
	if code == "" {
		v.AddError(field, "TOTP code is required")
		return
	}
	
	if len(code) != 6 {
		v.AddError(field, "TOTP code must be 6 digits")
		return
	}
	
	for _, char := range code {
		if !unicode.IsDigit(char) {
			v.AddError(field, "TOTP code must contain only digits")
			return
		}
	}
}

// ValidateAPIKeyName validates an API key name
func (v *Validator) ValidateAPIKeyName(field, name string) {
	if name == "" {
		v.AddError(field, "API key name is required")
		return
	}
	
	if len(name) < 3 {
		v.AddError(field, "API key name must be at least 3 characters")
		return
	}
	
	if len(name) > 100 {
		v.AddError(field, "API key name must be at most 100 characters")
		return
	}
	
	// Check for valid characters (letters, numbers, spaces, underscores, hyphens)
	for _, char := range name {
		if !unicode.IsLetter(char) && !unicode.IsNumber(char) && 
		   char != ' ' && char != '_' && char != '-' {
			v.AddError(field, "API key name contains invalid characters")
			return
		}
	}
}

// ValidateLimit validates pagination limit
func (v *Validator) ValidateLimit(field string, limit int, maxLimit int) {
	if limit < 1 {
		v.AddError(field, "limit must be at least 1")
		return
	}
	
	if limit > maxLimit {
		v.AddError(field, fmt.Sprintf("limit cannot exceed %d", maxLimit))
	}
}

// ValidateOffset validates pagination offset
func (v *Validator) ValidateOffset(field string, offset int) {
	if offset < 0 {
		v.AddError(field, "offset cannot be negative")
	}
}

// SendValidationErrors sends validation errors as JSON response
func SendValidationErrors(c *gin.Context, errors ValidationErrors) {
	c.JSON(400, gin.H{
		"error":   "Validation failed",
		"details": errors,
	})
}

// Example usage functions for common validation patterns



// ValidateCreateOrderRequest validates order creation data
func ValidateCreateOrderRequest(req CreateOrderRequest) ValidationErrors {
	validator := NewValidator()
	
	validator.ValidateMarketID("market_id", req.MarketID)
	validator.ValidateOrderSide("side", req.Side)
	validator.ValidateOrderType("type", req.Type)
	
	// Price validation depends on order type
	priceRequired := req.Type == "limit" || req.Type == "stop_limit"
	validator.ValidatePrice("price", req.Price, priceRequired)
	validator.ValidateSize("size", req.Size)
	
	return validator.GetErrors()
}

// Request structs with validation tags
type CreateOrderRequest struct {
	MarketID string `json:"market_id"`
	Side     int8   `json:"side"`
	Type     string `json:"type"`
	Price    string `json:"price"`
	Size     string `json:"size"`
} 