package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"bixor-engine/pkg/models"
	"gorm.io/gorm"
)

// SessionMiddleware handles session management
type SessionMiddleware struct {
	db *gorm.DB
}

// NewSessionMiddleware creates a new session middleware
func NewSessionMiddleware(db *gorm.DB) *SessionMiddleware {
	return &SessionMiddleware{
		db: db,
	}
}

// CreateSession creates a new user session
func (sm *SessionMiddleware) CreateSession(userID uint, token, refreshToken, ipAddress, userAgent string) (*models.UserSession, error) {
	// Hash the token for storage
	hasher := sha256.New()
	hasher.Write([]byte(token))
	tokenHash := hex.EncodeToString(hasher.Sum(nil))
	
	// Hash the refresh token
	hasher.Reset()
	hasher.Write([]byte(refreshToken))
	refreshTokenHash := hex.EncodeToString(hasher.Sum(nil))
	
	session := &models.UserSession{
		UserID:       userID,
		Token:        tokenHash,
		RefreshToken: refreshTokenHash,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		IsActive:     true,
		ExpiresAt:    time.Now().Add(24 * time.Hour), // 24 hours
	}
	
	if err := sm.db.Create(session).Error; err != nil {
		return nil, err
	}
	
	return session, nil
}

// ValidateSession validates a session token
func (sm *SessionMiddleware) ValidateSession(token string) (*models.UserSession, error) {
	hasher := sha256.New()
	hasher.Write([]byte(token))
	tokenHash := hex.EncodeToString(hasher.Sum(nil))
	
	var session models.UserSession
	err := sm.db.Preload("User").Where("token = ? AND is_active = ? AND expires_at > ?", 
		tokenHash, true, time.Now()).First(&session).Error
	
	if err != nil {
		return nil, err
	}
	
	return &session, nil
}

// RefreshSession refreshes a user session
func (sm *SessionMiddleware) RefreshSession(refreshToken string) (*models.UserSession, error) {
	hasher := sha256.New()
	hasher.Write([]byte(refreshToken))
	refreshTokenHash := hex.EncodeToString(hasher.Sum(nil))
	
	var session models.UserSession
	err := sm.db.Preload("User").Where("refresh_token = ? AND is_active = ? AND expires_at > ?", 
		refreshTokenHash, true, time.Now()).First(&session).Error
	
	if err != nil {
		return nil, err
	}
	
	// Update expiry
	session.ExpiresAt = time.Now().Add(24 * time.Hour)
	sm.db.Save(&session)
	
	return &session, nil
}

// InvalidateSession invalidates a session
func (sm *SessionMiddleware) InvalidateSession(token string) error {
	hasher := sha256.New()
	hasher.Write([]byte(token))
	tokenHash := hex.EncodeToString(hasher.Sum(nil))
	
	return sm.db.Model(&models.UserSession{}).
		Where("token = ?", tokenHash).
		Update("is_active", false).Error
}

// InvalidateAllUserSessions invalidates all sessions for a user
func (sm *SessionMiddleware) InvalidateAllUserSessions(userID uint) error {
	return sm.db.Model(&models.UserSession{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Update("is_active", false).Error
}

// CleanupExpiredSessions removes expired sessions
func (sm *SessionMiddleware) CleanupExpiredSessions() error {
	return sm.db.Where("expires_at < ?", time.Now()).Delete(&models.UserSession{}).Error
}

// GetActiveSessions gets active sessions for a user
func (sm *SessionMiddleware) GetActiveSessions(userID uint) ([]models.UserSession, error) {
	var sessions []models.UserSession
	err := sm.db.Where("user_id = ? AND is_active = ? AND expires_at > ?", 
		userID, true, time.Now()).
		Order("created_at DESC").
		Find(&sessions).Error
	
	return sessions, err
}

// SessionManagement middleware for tracking sessions
func (sm *SessionMiddleware) SessionManagement() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get session from context if user is authenticated
		user, exists := c.Get("user")
		if !exists {
			c.Next()
			return
		}
		
		userModel, ok := user.(*models.User)
		if !ok {
			c.Next()
			return
		}
		
		// Get token from header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}
		
		// Extract token
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.Next()
			return
		}
		
		token := tokenParts[1]
		
		// Validate session
		session, err := sm.ValidateSession(token)
		if err != nil {
			// Session invalid, but don't block request as JWT auth already passed
			c.Next()
			return
		}
		
		// Check if session IP matches current IP (optional security check)
		currentIP := c.ClientIP()
		if session.IPAddress != currentIP {
			// Log suspicious activity
			sm.logSuspiciousActivity(userModel.ID, session.IPAddress, currentIP)
		}
		
		// Set session info in context
		c.Set("session", session)
		c.Set("session_id", session.ID)
		
		c.Next()
	}
}

// RequireSession middleware that requires an active session
func (sm *SessionMiddleware) RequireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		session, exists := c.Get("session")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Active session required"})
			c.Abort()
			return
		}
		
		sessionModel, ok := session.(*models.UserSession)
		if !ok || !sessionModel.IsActive {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// SessionCleanup middleware that automatically cleans up expired sessions
func (sm *SessionMiddleware) SessionCleanup() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Run cleanup periodically (you might want to run this as a background job instead)
		go sm.CleanupExpiredSessions()
		c.Next()
	}
}

// logSuspiciousActivity logs suspicious session activity
func (sm *SessionMiddleware) logSuspiciousActivity(userID uint, sessionIP, currentIP string) {
	// In a real implementation, you'd log this to a security monitoring system
	// For now, we'll just log to database
	
	// You could create a SecurityEvent model for this
	// For now, we'll use LoginAttempt with a specific reason
	loginAttempt := models.LoginAttempt{
		Email:     "system", // You'd get this from user
		IPAddress: currentIP,
		Success:   false,
		Reason:    "IP_MISMATCH",
	}
	
	sm.db.Create(&loginAttempt)
}

// GetSessionStats returns session statistics
func (sm *SessionMiddleware) GetSessionStats(userID uint) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	// Count active sessions
	var activeCount int64
	err := sm.db.Model(&models.UserSession{}).
		Where("user_id = ? AND is_active = ? AND expires_at > ?", 
			userID, true, time.Now()).
		Count(&activeCount).Error
	
	if err != nil {
		return nil, err
	}
	
	// Get recent sessions
	var recentSessions []models.UserSession
	err = sm.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(5).
		Find(&recentSessions).Error
	
	if err != nil {
		return nil, err
	}
	
	stats["active_sessions"] = activeCount
	stats["recent_sessions"] = recentSessions
	
	return stats, nil
}

// DeviceFingerprint creates a simple device fingerprint
func (sm *SessionMiddleware) DeviceFingerprint(userAgent, acceptLanguage string) string {
	hasher := sha256.New()
	hasher.Write([]byte(userAgent + acceptLanguage))
	return hex.EncodeToString(hasher.Sum(nil))[:16] // First 16 chars
}

// IsNewDevice checks if this is a new device for the user
func (sm *SessionMiddleware) IsNewDevice(userID uint, userAgent, acceptLanguage string) (bool, error) {
	var count int64
	err := sm.db.Model(&models.UserSession{}).
		Where("user_id = ? AND user_agent = ?", userID, userAgent).
		Count(&count).Error
	
	if err != nil {
		return false, err
	}
	
	return count == 0, nil
} 