package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Trading  TradingConfig
}

type ServerConfig struct {
	Port         string
	Host         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	Environment  string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	MaxOpen  int
	MaxIdle  int
	MaxLife  time.Duration
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	Database int
	PoolSize int
}

type JWTConfig struct {
	SecretKey string
	ExpiresIn time.Duration
}

type TradingConfig struct {
	DefaultTakerFee    string
	DefaultMakerFee    string
	MinOrderSize       string
	MaxOrderSize       string
	OrderBookDepth     int
	CandlestickRetention time.Duration
}

func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			Host:         getEnv("SERVER_HOST", "localhost"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:  getDurationEnv("SERVER_IDLE_TIMEOUT", 60*time.Second),
			Environment:  getEnv("ENVIRONMENT", "development"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "bixor_db"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
			MaxOpen:  getIntEnv("DB_MAX_OPEN", 25),
			MaxIdle:  getIntEnv("DB_MAX_IDLE", 5),
			MaxLife:  getDurationEnv("DB_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			Database: getIntEnv("REDIS_DATABASE", 0),
			PoolSize: getIntEnv("REDIS_POOL_SIZE", 10),
		},
		JWT: JWTConfig{
			SecretKey: getEnv("JWT_SECRET_KEY", "bixor-engine-secret-key"),
			ExpiresIn: getDurationEnv("JWT_EXPIRES_IN", 24*time.Hour),
		},
		Trading: TradingConfig{
			DefaultTakerFee:      getEnv("DEFAULT_TAKER_FEE", "0.001"),
			DefaultMakerFee:      getEnv("DEFAULT_MAKER_FEE", "0.001"),
			MinOrderSize:         getEnv("MIN_ORDER_SIZE", "0.00000001"),
			MaxOrderSize:         getEnv("MAX_ORDER_SIZE", "1000000"),
			OrderBookDepth:       getIntEnv("ORDER_BOOK_DEPTH", 100),
			CandlestickRetention: getDurationEnv("CANDLESTICK_RETENTION", 30*24*time.Hour),
		},
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func (c *Config) GetDatabaseURL() string {
	return "postgres://" + c.Database.User + ":" + c.Database.Password + "@" + c.Database.Host + ":" + c.Database.Port + "/" + c.Database.DBName + "?sslmode=" + c.Database.SSLMode
}

func (c *Config) GetRedisURL() string {
	return c.Redis.Host + ":" + c.Redis.Port
}

func (c *Config) GetServerAddress() string {
	return c.Server.Host + ":" + c.Server.Port
}

func (c *Config) IsDevelopment() bool {
	return c.Server.Environment == "development"
}

func (c *Config) IsProduction() bool {
	return c.Server.Environment == "production"
} 