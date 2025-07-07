package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"bixor-engine/pkg/config"
	"bixor-engine/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Initialize database connection
func Initialize(cfg *config.Config) error {
	dsn := cfg.GetDatabaseURL()
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Connection pool configuration
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpen)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdle)
	sqlDB.SetConnMaxLifetime(cfg.Database.MaxLife)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	DB = db
	log.Println("Database connected successfully")
	return nil
}

// AutoMigrate runs database migrations
func AutoMigrate() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	err := DB.AutoMigrate(
		&models.User{},
		&models.Balance{},
		&models.Market{},
		&models.Order{},
		&models.Trade{},
		&models.MarketData{},
		// Auth models
		&models.UserSession{},
		&models.APIKey{},
		&models.TwoFactorAuth{},
		&models.LoginAttempt{},
		&models.RateLimit{},
		&models.UserPassword{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("Database migration completed successfully")
	return nil
}

// SeedData creates initial data for testing
func SeedData() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	// Create sample markets
	markets := []models.Market{
		{
			ID:             "BTC-USDT",
			BaseAsset:      "BTC",
			QuoteAsset:     "USDT",
			IsActive:       true,
			MinSize:        models.DecimalFromString("0.00001"),
			MaxSize:        models.DecimalFromString("1000"),
			PricePrecision: 2,
			SizePrecision:  8,
			TakerFee:       models.DecimalFromString("0.001"),
			MakerFee:       models.DecimalFromString("0.001"),
		},
		{
			ID:             "ETH-USDT",
			BaseAsset:      "ETH",
			QuoteAsset:     "USDT",
			IsActive:       true,
			MinSize:        models.DecimalFromString("0.0001"),
			MaxSize:        models.DecimalFromString("10000"),
			PricePrecision: 2,
			SizePrecision:  8,
			TakerFee:       models.DecimalFromString("0.001"),
			MakerFee:       models.DecimalFromString("0.001"),
		},
		{
			ID:             "ADA-USDT",
			BaseAsset:      "ADA",
			QuoteAsset:     "USDT",
			IsActive:       true,
			MinSize:        models.DecimalFromString("1"),
			MaxSize:        models.DecimalFromString("1000000"),
			PricePrecision: 4,
			SizePrecision:  2,
			TakerFee:       models.DecimalFromString("0.001"),
			MakerFee:       models.DecimalFromString("0.001"),
		},
	}

	for _, market := range markets {
		var existingMarket models.Market
		result := DB.Where("id = ?", market.ID).First(&existingMarket)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				if err := DB.Create(&market).Error; err != nil {
					return fmt.Errorf("failed to create market %s: %w", market.ID, err)
				}
				log.Printf("Created market: %s", market.ID)
			} else {
				return fmt.Errorf("failed to check market %s: %w", market.ID, result.Error)
			}
		}
	}

	// Create test users
	users := []models.User{
		{
			Email:      "alice@example.com",
			Username:   "alice",
			FirstName:  "Alice",
			LastName:   "Smith",
			IsActive:   true,
			IsVerified: true,
		},
		{
			Email:      "bob@example.com",
			Username:   "bob",
			FirstName:  "Bob",
			LastName:   "Johnson",
			IsActive:   true,
			IsVerified: true,
		},
	}

	for _, user := range users {
		var existingUser models.User
		result := DB.Where("email = ?", user.Email).First(&existingUser)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				if err := DB.Create(&user).Error; err != nil {
					return fmt.Errorf("failed to create user %s: %w", user.Email, err)
				}
				log.Printf("Created user: %s", user.Email)

				// Create initial balances
				balances := []models.Balance{
					{
						UserID:    user.ID,
						Asset:     "BTC",
						Available: models.DecimalFromString("1.0"),
						Locked:    models.DecimalFromString("0.0"),
					},
					{
						UserID:    user.ID,
						Asset:     "ETH",
						Available: models.DecimalFromString("10.0"),
						Locked:    models.DecimalFromString("0.0"),
					},
					{
						UserID:    user.ID,
						Asset:     "USDT",
						Available: models.DecimalFromString("10000.0"),
						Locked:    models.DecimalFromString("0.0"),
					},
				}

				for _, balance := range balances {
					balance.UserID = user.ID
					if err := DB.Create(&balance).Error; err != nil {
						return fmt.Errorf("failed to create balance for user %s: %w", user.Email, err)
					}
				}
			} else {
				return fmt.Errorf("failed to check user %s: %w", user.Email, result.Error)
			}
		}
	}

	log.Println("Database seeding completed successfully")
	return nil
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}

// Close closes the database connection
func Close() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// Health check for database
func HealthCheck() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
} 