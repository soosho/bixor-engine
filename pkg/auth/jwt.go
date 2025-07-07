package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"bixor-engine/pkg/models"
)

// JWTClaims represents JWT claims structure
type JWTClaims struct {
	UserID   uint             `json:"user_id"`
	Email    string           `json:"email"`
	Username string           `json:"username"`
	Role     models.UserRole  `json:"role"`
	jwt.RegisteredClaims
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	TokenType    string `json:"token_type"`
}

// JWTService handles JWT operations
type JWTService struct {
	secretKey       []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// NewJWTService creates a new JWT service
func NewJWTService(secretKey string, accessTTL, refreshTTL time.Duration) *JWTService {
	return &JWTService{
		secretKey:       []byte(secretKey),
		accessTokenTTL:  accessTTL,
		refreshTokenTTL: refreshTTL,
	}
}

// GenerateTokenPair generates access and refresh tokens
func (s *JWTService) GenerateTokenPair(user *models.User) (*TokenPair, error) {
	// Generate access token
	accessToken, accessExpiry, err := s.generateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := s.generateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessExpiry.Unix(),
		TokenType:    "Bearer",
	}, nil
}

// generateAccessToken creates a new access token
func (s *JWTService) generateAccessToken(user *models.User) (string, time.Time, error) {
	expiry := time.Now().Add(s.accessTokenTTL)
	
	claims := JWTClaims{
		UserID:   user.ID,
		Email:    user.Email,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   fmt.Sprintf("user:%d", user.ID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.secretKey)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiry, nil
}

// generateRefreshToken creates a new refresh token
func (s *JWTService) generateRefreshToken(userID uint) (string, error) {
	// Generate random bytes for refresh token
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	
	// Encode as base64 URL-safe string
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// ValidateToken validates and parses a JWT token
func (s *JWTService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// RefreshToken generates a new access token using refresh token
// NOTE: This function requires the refresh token to be validated externally
// The session middleware should validate the refresh token before calling this
func (s *JWTService) RefreshToken(refreshToken string, user *models.User) (*TokenPair, error) {
	// This function should only be called after refresh token validation
	// Validation is done in session middleware RefreshSession method
	return s.GenerateTokenPair(user)
}

// GenerateAPIKey generates a new API key pair
func GenerateAPIKey() (keyID, secret string, err error) {
	// Generate Key ID (16 bytes)
	keyIDBytes := make([]byte, 16)
	if _, err := rand.Read(keyIDBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate key ID: %w", err)
	}
	keyID = base64.URLEncoding.EncodeToString(keyIDBytes)

	// Generate Secret (32 bytes)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate secret: %w", err)
	}
	secret = base64.URLEncoding.EncodeToString(secretBytes)

	return keyID, secret, nil
} 