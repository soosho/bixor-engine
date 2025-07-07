package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"bixor-engine/pkg/auth"
	"bixor-engine/pkg/middleware"
	"bixor-engine/pkg/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthHandlers contains authentication-related handlers
type AuthHandlers struct {
	db                *gorm.DB
	jwtService        *auth.JWTService
	totpService       *auth.TOTPService
	authMiddleware    *middleware.AuthMiddleware
	sessionMiddleware *middleware.SessionMiddleware
}

// NewAuthHandlers creates new authentication handlers
func NewAuthHandlers(db *gorm.DB, jwtService *auth.JWTService, totpService *auth.TOTPService, 
	authMiddleware *middleware.AuthMiddleware, sessionMiddleware *middleware.SessionMiddleware) *AuthHandlers {
	return &AuthHandlers{
		db:                db,
		jwtService:        jwtService,
		totpService:       totpService,
		authMiddleware:    authMiddleware,
		sessionMiddleware: sessionMiddleware,
	}
}

// RegisterRequest represents user registration request
type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Username  string `json:"username" binding:"required,min=3,max=50"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// LoginRequest represents user login request
type LoginRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required"`
	TotpCode   string `json:"totp_code,omitempty"`
	BackupCode string `json:"backup_code,omitempty"`
}

// Register handles user registration
func (ah *AuthHandlers) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := ah.db.Where("email = ? OR username = ?", req.Email, req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Create user
	user := models.User{
		Email:     req.Email,
		Username:  req.Username,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      models.RoleUser,
		IsActive:  true,
		IsVerified: false,
	}

	if err := ah.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Store password hash
	userPassword := models.UserPassword{
		UserID:       user.ID,
		PasswordHash: string(hashedPassword),
	}

	if err := ah.db.Create(&userPassword).Error; err != nil {
		// If password storage fails, delete the user to maintain consistency
		ah.db.Delete(&user)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store password"})
		return
	}

	// Log registration
	ah.authMiddleware.LogLogin(req.Email, c.ClientIP(), c.Request.UserAgent(), true, "REGISTRATION")

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user_id": user.ID,
	})
}

// Login handles user login
func (ah *AuthHandlers) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user
	var user models.User
	if err := ah.db.Where("email = ? AND is_active = ?", req.Email, true).First(&user).Error; err != nil {
		ah.authMiddleware.LogLogin(req.Email, c.ClientIP(), c.Request.UserAgent(), false, "USER_NOT_FOUND")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Get user password
	var userPassword models.UserPassword
	if err := ah.db.Where("user_id = ?", user.ID).First(&userPassword).Error; err != nil {
		ah.authMiddleware.LogLogin(req.Email, c.ClientIP(), c.Request.UserAgent(), false, "PASSWORD_NOT_FOUND")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(userPassword.PasswordHash), []byte(req.Password)); err != nil {
		ah.authMiddleware.LogLogin(req.Email, c.ClientIP(), c.Request.UserAgent(), false, "INVALID_PASSWORD")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check if 2FA is enabled
	var twoFA models.TwoFactorAuth
	has2FA := ah.db.Where("user_id = ? AND is_enabled = ?", user.ID, true).First(&twoFA).Error == nil

	if has2FA {
		if req.TotpCode == "" && req.BackupCode == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "2FA code or backup code required",
				"requires_2fa": true,
			})
			return
		}

		// Try TOTP code first
		validAuth := false
		if req.TotpCode != "" {
			if ah.totpService.ValidateToken(twoFA.Secret, req.TotpCode) {
				validAuth = true
			}
		}

		// Try backup code if TOTP failed or wasn't provided
		if !validAuth && req.BackupCode != "" {
			isValid, updatedCodes, err := auth.ValidateBackupCode(twoFA.BackupCodes, req.BackupCode)
			if err == nil && isValid {
				validAuth = true
				// Update backup codes (mark as used)
				backupCodesJSON, _ := json.Marshal(updatedCodes)
				ah.db.Model(&twoFA).Update("backup_codes", string(backupCodesJSON))
			}
		}

		if !validAuth {
			ah.authMiddleware.LogLogin(req.Email, c.ClientIP(), c.Request.UserAgent(), false, "INVALID_2FA")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid 2FA or backup code"})
			return
		}

		// Update last used
		now := time.Now()
		ah.db.Model(&twoFA).Update("last_used_at", &now)
	}

	// Generate JWT tokens
	tokenPair, err := ah.jwtService.GenerateTokenPair(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	// Create session
	session, err := ah.sessionMiddleware.CreateSession(
		user.ID,
		tokenPair.AccessToken,
		tokenPair.RefreshToken,
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	// Log successful login
	ah.authMiddleware.LogLogin(req.Email, c.ClientIP(), c.Request.UserAgent(), true, "SUCCESS")

	response := gin.H{
		"message": "Login successful",
		"user": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"username":   user.Username,
			"role":       user.Role,
			"is_verified": user.IsVerified,
		},
		"tokens": tokenPair,
	}

	// Only include session_id if session was created successfully
	if err == nil && session != nil {
		response["session_id"] = session.ID
	}

	c.JSON(http.StatusOK, response)
}

// Logout handles user logout
func (ah *AuthHandlers) Logout(c *gin.Context) {
	// Get user from context
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Get logout options from request body (optional)
	var req struct {
		LogoutAll       bool `json:"logout_all,omitempty"`        // Logout from all devices
		RevokeAPIKeys   bool `json:"revoke_api_keys,omitempty"`   // Revoke API keys
	}
	// Ignore binding errors for optional fields
	c.ShouldBindJSON(&req)

	// Get token from header for current session invalidation
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) == 2 && tokenParts[0] == "Bearer" {
			token := tokenParts[1]
			
			if req.LogoutAll {
				// Invalidate all user sessions
				if err := ah.sessionMiddleware.InvalidateAllUserSessions(user.ID); err != nil {
					// Log error but continue with logout
					ah.authMiddleware.LogLogin(user.Email, c.ClientIP(), c.Request.UserAgent(), false, "LOGOUT_ALL_SESSIONS_FAILED")
				}
			} else {
				// Invalidate only current session
				if err := ah.sessionMiddleware.InvalidateSession(token); err != nil {
					// Log error but continue with logout
					ah.authMiddleware.LogLogin(user.Email, c.ClientIP(), c.Request.UserAgent(), false, "LOGOUT_SESSION_FAILED")
				}
			}
		}
	}

	// Revoke API keys if requested
	if req.RevokeAPIKeys {
		if err := ah.db.Model(&models.APIKey{}).
			Where("user_id = ? AND is_active = ?", user.ID, true).
			Update("is_active", false).Error; err != nil {
			// Log error but continue with logout
			ah.authMiddleware.LogLogin(user.Email, c.ClientIP(), c.Request.UserAgent(), false, "LOGOUT_REVOKE_API_KEYS_FAILED")
		}
	}

	// Log successful logout
	ah.authMiddleware.LogLogin(user.Email, c.ClientIP(), c.Request.UserAgent(), true, "LOGOUT")

	response := gin.H{"message": "Logout successful"}
	if req.LogoutAll {
		response["logged_out_all_devices"] = true
	}
	if req.RevokeAPIKeys {
		response["api_keys_revoked"] = true
	}

	c.JSON(http.StatusOK, response)
}

// RefreshToken handles token refresh
func (ah *AuthHandlers) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate refresh token and get user
	session, err := ah.sessionMiddleware.RefreshSession(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	// Generate new token pair
	tokenPair, err := ah.jwtService.GenerateTokenPair(&session.User)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tokens": tokenPair,
	})
}

// Enable2FA handles 2FA setup
func (ah *AuthHandlers) Enable2FA(c *gin.Context) {
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Check if 2FA is already enabled
	var existing models.TwoFactorAuth
	if err := ah.db.Where("user_id = ?", user.ID).First(&existing).Error; err == nil && existing.IsEnabled {
		c.JSON(http.StatusConflict, gin.H{"error": "2FA already enabled"})
		return
	}

	// Generate TOTP secret
	key, err := ah.totpService.GenerateSecret(user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate secret"})
		return
	}

	// Generate QR code URL
	qrURL, err := ah.totpService.GenerateQRCode(key.Secret(), user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR code"})
		return
	}

	// Generate backup codes
	backupCodes, err := auth.GenerateBackupCodes(8)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate backup codes"})
		return
	}

	backupCodesJSON, _ := json.Marshal(backupCodes)

	// Store 2FA settings (not enabled yet)
	twoFA := models.TwoFactorAuth{
		UserID:      user.ID,
		Secret:      key.Secret(),
		BackupCodes: string(backupCodesJSON),
		IsEnabled:   false,
	}

	if err := ah.db.Create(&twoFA).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store 2FA settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"secret":       key.Secret(),
		"qr_url":       qrURL,
		"backup_codes": backupCodes,
		"message":      "2FA setup initiated. Please verify with a TOTP code to complete setup.",
	})
}

// Verify2FA handles 2FA verification and enables it
func (ah *AuthHandlers) Verify2FA(c *gin.Context) {
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var req struct {
		TotpCode string `json:"totp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get 2FA settings
	var twoFA models.TwoFactorAuth
	if err := ah.db.Where("user_id = ?", user.ID).First(&twoFA).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "2FA not set up"})
		return
	}

	// Validate TOTP code
	if !ah.totpService.ValidateToken(twoFA.Secret, req.TotpCode) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid TOTP code"})
		return
	}

	// Enable 2FA
	twoFA.IsEnabled = true
	if err := ah.db.Save(&twoFA).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable 2FA"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "2FA enabled successfully"})
}

// Disable2FA handles 2FA disabling
func (ah *AuthHandlers) Disable2FA(c *gin.Context) {
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var req struct {
		TotpCode string `json:"totp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get 2FA settings
	var twoFA models.TwoFactorAuth
	if err := ah.db.Where("user_id = ? AND is_enabled = ?", user.ID, true).First(&twoFA).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "2FA not enabled"})
		return
	}

	// Validate TOTP code
	if !ah.totpService.ValidateToken(twoFA.Secret, req.TotpCode) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid TOTP code"})
		return
	}

	// Disable 2FA
	if err := ah.db.Delete(&twoFA).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable 2FA"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "2FA disabled successfully"})
}

// CreateAPIKey handles API key creation
func (ah *AuthHandlers) CreateAPIKey(c *gin.Context) {
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var req struct {
		Name        string   `json:"name" binding:"required"`
		Permissions []string `json:"permissions"`
		ExpiresAt   *time.Time `json:"expires_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate API key
	keyID, secret, err := auth.GenerateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate API key"})
		return
	}

	// Hash the secret
	hasher := sha256.New()
	hasher.Write([]byte(secret))
	secretHash := hex.EncodeToString(hasher.Sum(nil))

	// Serialize permissions
	permissionsJSON, _ := json.Marshal(req.Permissions)

	// Create API key record
	apiKey := models.APIKey{
		UserID:      user.ID,
		Name:        req.Name,
		KeyID:       keyID,
		SecretHash:  secretHash,
		Permissions: string(permissionsJSON),
		IsActive:    true,
		ExpiresAt:   req.ExpiresAt,
	}

	if err := ah.db.Create(&apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create API key"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "API key created successfully",
		"key_id":  keyID,
		"secret":  secret,
		"warning": "Save the secret securely. It will not be shown again.",
	})
}

// ListAPIKeys handles listing user's API keys
func (ah *AuthHandlers) ListAPIKeys(c *gin.Context) {
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var apiKeys []models.APIKey
	if err := ah.db.Where("user_id = ?", user.ID).Find(&apiKeys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch API keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"api_keys": apiKeys,
	})
}

// RevokeAPIKey handles API key revocation
func (ah *AuthHandlers) RevokeAPIKey(c *gin.Context) {
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	keyID := c.Param("key_id")
	if keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Key ID required"})
		return
	}

	// Find and deactivate API key
	result := ah.db.Model(&models.APIKey{}).
		Where("user_id = ? AND key_id = ?", user.ID, keyID).
		Update("is_active", false)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke API key"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key revoked successfully"})
}

// GetProfile handles getting user profile
func (ah *AuthHandlers) GetProfile(c *gin.Context) {
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Get 2FA status
	var twoFA models.TwoFactorAuth
	has2FA := ah.db.Where("user_id = ? AND is_enabled = ?", user.ID, true).First(&twoFA).Error == nil

	// Get session stats
	sessionStats, _ := ah.sessionMiddleware.GetSessionStats(user.ID)

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"username":   user.Username,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
			"role":       user.Role,
			"is_active":  user.IsActive,
			"is_verified": user.IsVerified,
			"created_at": user.CreatedAt,
		},
		"security": gin.H{
			"has_2fa":     has2FA,
			"sessions":    sessionStats,
		},
	})
} 