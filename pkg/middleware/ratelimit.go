package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"bixor-engine/pkg/cache"
	"bixor-engine/pkg/models"
	"gorm.io/gorm"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Requests   int           // Number of requests
	Window     time.Duration // Time window
	KeyFunc    func(c *gin.Context) string // Function to generate rate limit key
	Message    string        // Error message to return
	StatusCode int           // HTTP status code to return
}

// Default rate limiting configurations
var (
	DefaultRateLimit = RateLimitConfig{
		Requests:   100,
		Window:     time.Minute,
		KeyFunc:    func(c *gin.Context) string { return c.ClientIP() },
		Message:    "Too many requests, please try again later",
		StatusCode: http.StatusTooManyRequests,
	}
	
	PublicRateLimit = RateLimitConfig{
		Requests:   1000,
		Window:     time.Minute,
		KeyFunc:    func(c *gin.Context) string { return c.ClientIP() },
		Message:    "Too many requests, please try again later",
		StatusCode: http.StatusTooManyRequests,
	}
	
	TradingRateLimit = RateLimitConfig{
		Requests:   10,
		Window:     time.Second,
		KeyFunc:    func(c *gin.Context) string { 
			if userID, exists := c.Get("user_id"); exists {
				return fmt.Sprintf("user:%v", userID)
			}
			return c.ClientIP()
		},
		Message:    "Trading rate limit exceeded",
		StatusCode: http.StatusTooManyRequests,
	}
)

// RateLimitMiddleware handles rate limiting
type RateLimitMiddleware struct {
	cache *cache.RedisCache
	db    *gorm.DB
}

// NewRateLimitMiddleware creates a new rate limiting middleware
func NewRateLimitMiddleware(cache *cache.RedisCache, db *gorm.DB) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		cache: cache,
		db:    db,
	}
}

// IPRateLimit creates a rate limiting middleware for IP addresses
func (rl *RateLimitMiddleware) IPRateLimit(config RateLimitConfig) gin.HandlerFunc {
	return rl.RateLimit(config)
}

// PublicRateLimit creates a rate limiting middleware for public endpoints
func (rl *RateLimitMiddleware) PublicRateLimit() gin.HandlerFunc {
	return rl.RateLimit(PublicRateLimit)
}

// TradingRateLimit creates a rate limiting middleware for trading endpoints
func (rl *RateLimitMiddleware) TradingRateLimit() gin.HandlerFunc {
	return rl.RateLimit(TradingRateLimit)
}

// RateLimit creates a rate limiting middleware with the given configuration
func (rl *RateLimitMiddleware) RateLimit(config RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := config.KeyFunc(c)
		rateLimitKey := fmt.Sprintf("rate_limit:%s", key)
		
		// Try Redis first for better performance
		if rl.cache != nil {
			allowed, err := rl.checkRateLimitRedis(rateLimitKey, config)
			if err == nil {
				if !allowed {
					c.JSON(config.StatusCode, gin.H{"error": config.Message})
					c.Abort()
					return
				}
				c.Next()
				return
			}
			// If Redis fails, fall back to database
		}
		
		// Fallback to database rate limiting
		allowed, err := rl.checkRateLimitDB(key, config)
		if err != nil {
			// If rate limiting fails, we'll allow the request but log the error
			// This ensures the service doesn't become unavailable due to rate limiting issues
			c.Next()
			return
		}
		
		if !allowed {
			c.JSON(config.StatusCode, gin.H{"error": config.Message})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// checkRateLimitRedis checks rate limiting using Redis
func (rl *RateLimitMiddleware) checkRateLimitRedis(key string, config RateLimitConfig) (bool, error) {
	// Use Redis sliding window counter
	now := time.Now().Unix()
	expiredTime := now - int64(config.Window.Seconds())
	
	// Remove expired entries
	_, err := rl.cache.Client().ZRemRangeByScore(rl.cache.Context(), key, "0", strconv.FormatInt(expiredTime, 10)).Result()
	if err != nil {
		return false, err
	}
	
	// Count current requests
	count, err := rl.cache.Client().ZCard(rl.cache.Context(), key).Result()
	if err != nil {
		return false, err
	}
	
	// Check if limit exceeded
	if count >= int64(config.Requests) {
		return false, nil
	}
	
	// Add current request
	err = rl.cache.Client().ZAdd(rl.cache.Context(), key, &redis.Z{
		Score:  float64(now),
		Member: fmt.Sprintf("%d-%d", now, time.Now().UnixNano()),
	}).Err()
	if err != nil {
		return false, err
	}
	
	// Set expiration
	err = rl.cache.Client().Expire(rl.cache.Context(), key, config.Window).Err()
	if err != nil {
		return false, err
	}
	
	return true, nil
}

// checkRateLimitDB checks rate limiting using database
func (rl *RateLimitMiddleware) checkRateLimitDB(key string, config RateLimitConfig) (bool, error) {
	now := time.Now()
	windowStart := now.Add(-config.Window)
	
	// Clean up old entries
	rl.db.Where("key = ? AND window_start < ?", key, windowStart).Delete(&models.RateLimit{})
	
	// Get current rate limit record
	var rateLimit models.RateLimit
	result := rl.db.Where("key = ? AND window_start >= ?", key, windowStart).First(&rateLimit)
	
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Create new rate limit record
			rateLimit = models.RateLimit{
				Key:         key,
				Count:       1,
				WindowStart: now,
			}
			if err := rl.db.Create(&rateLimit).Error; err != nil {
				return false, err
			}
			return true, nil
		}
		return false, result.Error
	}
	
	// Check if rate limit is exceeded
	if rateLimit.Count >= config.Requests {
		return false, nil
	}
	
	// Increment count
	rateLimit.Count++
	if err := rl.db.Save(&rateLimit).Error; err != nil {
		return false, err
	}
	
	return true, nil
}

// UserRateLimit creates a rate limiting middleware for authenticated users
func (rl *RateLimitMiddleware) UserRateLimit(requests int, window time.Duration) gin.HandlerFunc {
	config := RateLimitConfig{
		Requests: requests,
		Window:   window,
		KeyFunc: func(c *gin.Context) string {
			if userID, exists := c.Get("user_id"); exists {
				return fmt.Sprintf("user:%v", userID)
			}
			return c.ClientIP()
		},
		Message:    "Rate limit exceeded",
		StatusCode: http.StatusTooManyRequests,
	}
	return rl.RateLimit(config)
}

// APIKeyRateLimit creates a rate limiting middleware for API keys
func (rl *RateLimitMiddleware) APIKeyRateLimit(requests int, window time.Duration) gin.HandlerFunc {
	config := RateLimitConfig{
		Requests: requests,
		Window:   window,
		KeyFunc: func(c *gin.Context) string {
			if apiKey, exists := c.Get("api_key"); exists {
				if keyModel, ok := apiKey.(*models.APIKey); ok {
					return fmt.Sprintf("api_key:%s", keyModel.KeyID)
				}
			}
			return c.ClientIP()
		},
		Message:    "API rate limit exceeded",
		StatusCode: http.StatusTooManyRequests,
	}
	return rl.RateLimit(config)
}

// GetRateLimitStatus returns the current rate limit status for a key
func (rl *RateLimitMiddleware) GetRateLimitStatus(key string, config RateLimitConfig) (int, int, error) {
	rateLimitKey := fmt.Sprintf("rate_limit:%s", key)
	
	// Try Redis first
	if rl.cache != nil {
		expiredTime := time.Now().Add(-config.Window).Unix()
		
		// Remove expired entries
		_, err := rl.cache.Client().ZRemRangeByScore(rl.cache.Context(), rateLimitKey, "0", strconv.FormatInt(expiredTime, 10)).Result()
		if err == nil {
			// Get count
			count, err := rl.cache.Client().ZCard(rl.cache.Context(), rateLimitKey).Result()
			if err == nil {
				return int(count), config.Requests, nil
			}
		}
	}
	
	// Fallback to database
	windowStart := time.Now().Add(-config.Window)
	var rateLimit models.RateLimit
	
	result := rl.db.Where("key = ? AND window_start >= ?", key, windowStart).First(&rateLimit)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return 0, config.Requests, nil
		}
		return 0, 0, result.Error
	}
	
	return rateLimit.Count, config.Requests, nil
} 