package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"bixor-engine/pkg/auth"
	"bixor-engine/pkg/models"
	"gorm.io/gorm"
)

// AuthMiddleware handles authentication
type AuthMiddleware struct {
	jwtService *auth.JWTService
	db         *gorm.DB
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(jwtService *auth.JWTService, db *gorm.DB) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
		db:         db,
	}
}

// JWTAuth middleware for JWT authentication
func (am *AuthMiddleware) JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Check if it's a Bearer token
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		token := tokenParts[1]
		claims, err := am.jwtService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Get user from database
		var user models.User
		if err := am.db.First(&user, claims.UserID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		// Check if user is active
		if !user.IsActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "User account is disabled"})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user", &user)
		c.Set("user_id", claims.UserID)
		c.Set("user_role", claims.Role)
		c.Next()
	}
}

// APIKeyAuth middleware for API key authentication
func (am *AuthMiddleware) APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		apiSecret := c.GetHeader("X-API-Secret")

		if apiKey == "" || apiSecret == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key and secret required"})
			c.Abort()
			return
		}

		// Get API key from database
		var apiKeyModel models.APIKey
		if err := am.db.Preload("User").Where("key_id = ? AND is_active = ?", apiKey, true).First(&apiKeyModel).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		// Check if API key is expired
		if apiKeyModel.ExpiresAt != nil && apiKeyModel.ExpiresAt.Before(time.Now()) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key expired"})
			c.Abort()
			return
		}

		// Validate API secret
		if !am.validateAPISecret(apiSecret, apiKeyModel.SecretHash) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API secret"})
			c.Abort()
			return
		}

		// Check if user is active
		if !apiKeyModel.User.IsActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "User account is disabled"})
			c.Abort()
			return
		}

		// Update last used timestamp
		now := time.Now()
		am.db.Model(&apiKeyModel).Update("last_used_at", &now)

		// Set user info in context
		c.Set("user", &apiKeyModel.User)
		c.Set("user_id", apiKeyModel.UserID)
		c.Set("user_role", apiKeyModel.User.Role)
		c.Set("api_key", &apiKeyModel)
		c.Next()
	}
}

// OptionalAuth middleware that allows both authenticated and unauthenticated access
func (am *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try JWT first
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenParts := strings.Split(authHeader, " ")
			if len(tokenParts) == 2 && tokenParts[0] == "Bearer" {
				token := tokenParts[1]
				if claims, err := am.jwtService.ValidateToken(token); err == nil {
					var user models.User
					if err := am.db.First(&user, claims.UserID).Error; err == nil && user.IsActive {
						c.Set("user", &user)
						c.Set("user_id", claims.UserID)
						c.Set("user_role", claims.Role)
						c.Next()
						return
					}
				}
			}
		}

		// Try API key
		apiKey := c.GetHeader("X-API-Key")
		apiSecret := c.GetHeader("X-API-Secret")
		if apiKey != "" && apiSecret != "" {
			var apiKeyModel models.APIKey
			if err := am.db.Preload("User").Where("key_id = ? AND is_active = ?", apiKey, true).First(&apiKeyModel).Error; err == nil {
				if (apiKeyModel.ExpiresAt == nil || apiKeyModel.ExpiresAt.After(time.Now())) &&
					am.validateAPISecret(apiSecret, apiKeyModel.SecretHash) &&
					apiKeyModel.User.IsActive {
					
					// Update last used timestamp
					now := time.Now()
					am.db.Model(&apiKeyModel).Update("last_used_at", &now)
					
					c.Set("user", &apiKeyModel.User)
					c.Set("user_id", apiKeyModel.UserID)
					c.Set("user_role", apiKeyModel.User.Role)
					c.Set("api_key", &apiKeyModel)
					c.Next()
					return
				}
			}
		}

		// No authentication provided, continue without user context
		c.Next()
	}
}

// validateAPISecret validates API secret using SHA256 hash comparison
func (am *AuthMiddleware) validateAPISecret(providedSecret, storedHash string) bool {
	// Hash the provided secret using SHA256 (same as in CreateAPIKey)
	hasher := sha256.New()
	hasher.Write([]byte(providedSecret))
	providedHash := hex.EncodeToString(hasher.Sum(nil))
	
	// Compare hashes using constant-time comparison
	return hmac.Equal([]byte(providedHash), []byte(storedHash))
}

// RequireRole middleware that requires specific user roles
func RequireRole(roles ...models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		role, ok := userRole.(models.UserRole)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid user role"})
			c.Abort()
			return
		}

		// Check if user has required role
		for _, requiredRole := range roles {
			if role == requiredRole {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		c.Abort()
	}
}

// RequireAdmin middleware that requires admin role
func RequireAdmin() gin.HandlerFunc {
	return RequireRole(models.RoleAdmin, models.RoleSuper)
}

// RequireTrader middleware that requires trader role or higher
func RequireTrader() gin.HandlerFunc {
	return RequireRole(models.RoleTrader, models.RoleAdmin, models.RoleSuper)
}

// RequireVerified middleware that requires verified user
func RequireVerified() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		userModel, ok := user.(*models.User)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid user"})
			c.Abort()
			return
		}

		if !userModel.IsVerified {
			c.JSON(http.StatusForbidden, gin.H{"error": "User account must be verified"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// LogLogin logs login attempts
func (am *AuthMiddleware) LogLogin(email, ipAddress, userAgent string, success bool, reason string) {
	loginAttempt := models.LoginAttempt{
		Email:     email,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   success,
		Reason:    reason,
	}
	am.db.Create(&loginAttempt)
}

// GetUserFromContext gets user from gin context
func GetUserFromContext(c *gin.Context) (*models.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}
	
	userModel, ok := user.(*models.User)
	return userModel, ok
}

// GetUserIDFromContext gets user ID from gin context
func GetUserIDFromContext(c *gin.Context) (uint, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	
	id, ok := userID.(uint)
	return id, ok
} 