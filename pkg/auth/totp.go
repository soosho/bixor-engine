package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/pbkdf2"
)

// TOTPService handles TOTP operations
type TOTPService struct {
	issuer string
}

// NewTOTPService creates a new TOTP service
func NewTOTPService(issuer string) *TOTPService {
	return &TOTPService{
		issuer: issuer,
	}
}

// GenerateSecret generates a new TOTP secret for a user
func (s *TOTPService) GenerateSecret(email string) (*otp.Key, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: email,
		SecretSize:  32,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP secret: %w", err)
	}
	return key, nil
}

// GenerateQRCode generates a QR code URL for the TOTP secret
func (s *TOTPService) GenerateQRCode(secret, email string) (string, error) {
	// Create the otpauth URL
	params := url.Values{}
	params.Add("secret", secret)
	params.Add("issuer", s.issuer)
	
	otpauthURL := fmt.Sprintf("otpauth://totp/%s:%s?%s", 
		url.QueryEscape(s.issuer), 
		url.QueryEscape(email), 
		params.Encode())
	
	// Generate QR code URL (you can use a service like Google Charts)
	qrURL := fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=%s", 
		url.QueryEscape(otpauthURL))
	
	return qrURL, nil
}

// ValidateToken validates a TOTP token
func (s *TOTPService) ValidateToken(secret, token string) bool {
	return totp.Validate(token, secret)
}

// ValidateTokenWithWindow validates a TOTP token with a time window
func (s *TOTPService) ValidateTokenWithWindow(secret, token string, window int) bool {
	// Parse the token
	tokenInt, err := strconv.ParseInt(token, 10, 32)
	if err != nil {
		return false
	}

	// Get current time
	now := time.Now()
	
	// Check current time and time windows
	for i := -window; i <= window; i++ {
		testTime := now.Add(time.Duration(i) * 30 * time.Second)
		expectedToken, err := totp.GenerateCodeCustom(secret, testTime, totp.ValidateOpts{
			Period:    30,
			Skew:      0,
			Digits:    6,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			continue
		}
		
		expectedInt, err := strconv.ParseInt(expectedToken, 10, 32)
		if err != nil {
			continue
		}
		
		if tokenInt == expectedInt {
			return true
		}
	}
	
	return false
}

// BackupCode represents a backup code for 2FA
type BackupCode struct {
	Code    string    `json:"code"`
	Used    bool      `json:"used"`
	UsedAt  *time.Time `json:"used_at,omitempty"`
}

// GenerateBackupCodes generates backup codes for 2FA
func GenerateBackupCodes(count int) ([]BackupCode, error) {
	codes := make([]BackupCode, count)
	
	for i := 0; i < count; i++ {
		// Generate 8-character random code
		code, err := generateRandomCode(8)
		if err != nil {
			return nil, fmt.Errorf("failed to generate backup code: %w", err)
		}
		
		codes[i] = BackupCode{
			Code: code,
			Used: false,
		}
	}
	
	return codes, nil
}

// generateRandomCode generates a random alphanumeric code
func generateRandomCode(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	
	for i, b := range bytes {
		bytes[i] = charset[b%byte(len(charset))]
	}
	
	return string(bytes), nil
}

// ValidateBackupCode validates a backup code
func ValidateBackupCode(storedCodes string, inputCode string) (bool, []BackupCode, error) {
	var codes []BackupCode
	if err := json.Unmarshal([]byte(storedCodes), &codes); err != nil {
		return false, nil, fmt.Errorf("failed to unmarshal backup codes: %w", err)
	}
	
	// Clean input code
	inputCode = strings.ToUpper(strings.TrimSpace(inputCode))
	
	for i, code := range codes {
		if code.Code == inputCode && !code.Used {
			// Mark as used
			codes[i].Used = true
			now := time.Now()
			codes[i].UsedAt = &now
			return true, codes, nil
		}
	}
	
	return false, codes, nil
}

// EncryptSecret encrypts a TOTP secret using AES-256-GCM
func EncryptSecret(secret, password string) (string, error) {
	// Derive key from password using PBKDF2
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}
	
	key := pbkdf2.Key([]byte(password), salt, 10000, 32, sha256.New)
	
	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	// Encrypt the secret
	ciphertext := gcm.Seal(nonce, nonce, []byte(secret), nil)
	
	// Combine salt and ciphertext
	result := append(salt, ciphertext...)
	
	return base64.StdEncoding.EncodeToString(result), nil
}

// DecryptSecret decrypts a TOTP secret using AES-256-GCM
func DecryptSecret(encryptedSecret, password string) (string, error) {
	// Decode base64
	data, err := base64.StdEncoding.DecodeString(encryptedSecret)
	if err != nil {
		return "", fmt.Errorf("failed to decode secret: %w", err)
	}
	
	if len(data) < 32 {
		return "", fmt.Errorf("invalid encrypted secret length")
	}
	
	// Extract salt and ciphertext
	salt := data[:32]
	ciphertext := data[32:]
	
	// Derive key from password using PBKDF2
	key := pbkdf2.Key([]byte(password), salt, 10000, 32, sha256.New)
	
	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Check minimum length
	if len(ciphertext) < gcm.NonceSize() {
		return "", fmt.Errorf("invalid ciphertext length")
	}
	
	// Extract nonce and encrypted data
	nonce := ciphertext[:gcm.NonceSize()]
	encryptedData := ciphertext[gcm.NonceSize():]
	
	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret: %w", err)
	}
	
	return string(plaintext), nil
}

// GenerateRecoveryCodes generates recovery codes as an alternative to backup codes
func GenerateRecoveryCodes(count int) ([]string, error) {
	codes := make([]string, count)
	
	for i := 0; i < count; i++ {
		// Generate a longer recovery code (16 characters)
		code, err := generateRandomCode(16)
		if err != nil {
			return nil, fmt.Errorf("failed to generate recovery code: %w", err)
		}
		
		// Format as XXXX-XXXX-XXXX-XXXX
		formatted := fmt.Sprintf("%s-%s-%s-%s", 
			code[0:4], code[4:8], code[8:12], code[12:16])
		codes[i] = formatted
	}
	
	return codes, nil
} 