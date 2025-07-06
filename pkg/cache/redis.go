package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"https://github.com/soosho/bixor-engine/pkg/config"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

var (
	RedisClient *redis.Client
	ctx         = context.Background()
)

// Initialize Redis connection
func Initialize(cfg *config.Config) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:         cfg.GetRedisURL(),
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.Database,
		PoolSize:     cfg.Redis.PoolSize,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	})

	// Test connection
	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logrus.Info("Redis connected successfully")
	return nil
}

// Cache keys constants
const (
	KeyOrderBookDepth = "orderbook:depth:%s"          // orderbook:depth:BTC-USDT
	KeyMarketData     = "market:data:%s"              // market:data:BTC-USDT
	KeyUserBalances   = "user:balances:%d"            // user:balances:123
	KeyRecentTrades   = "trades:recent:%s"            // trades:recent:BTC-USDT
	KeyMarketStats    = "market:stats:%s"             // market:stats:BTC-USDT
	KeyOrderBookFull  = "orderbook:full:%s"           // orderbook:full:BTC-USDT
	KeyUserOrders     = "user:orders:%d"              // user:orders:123
	KeyTradingPairs   = "trading:pairs"               // trading:pairs
	KeyKlineData      = "kline:%s:%s"                 // kline:BTC-USDT:1m
)

// Cache expiration times
const (
	ExpireOrderBookDepth = 1 * time.Second
	ExpireMarketData     = 5 * time.Second
	ExpireUserBalances   = 30 * time.Second
	ExpireRecentTrades   = 10 * time.Second
	ExpireMarketStats    = 60 * time.Second
	ExpireOrderBookFull  = 2 * time.Second
	ExpireUserOrders     = 10 * time.Second
	ExpireTradingPairs   = 300 * time.Second
	ExpireKlineData      = 60 * time.Second
)

// Set stores a value in Redis with expiration
func Set(key string, value interface{}, expiration time.Duration) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	err = RedisClient.Set(ctx, key, jsonValue, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}

	return nil
}

// Get retrieves a value from Redis
func Get(key string, dest interface{}) error {
	val, err := RedisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("key %s not found", key)
		}
		return fmt.Errorf("failed to get key %s: %w", key, err)
	}

	err = json.Unmarshal([]byte(val), dest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal value for key %s: %w", key, err)
	}

	return nil
}

// Delete removes a key from Redis
func Delete(key string) error {
	err := RedisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}
	return nil
}

// Exists checks if a key exists in Redis
func Exists(key string) bool {
	result, err := RedisClient.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return result > 0
}

// SetNX sets a key only if it doesn't exist
func SetNX(key string, value interface{}, expiration time.Duration) (bool, error) {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("failed to marshal value: %w", err)
	}

	result, err := RedisClient.SetNX(ctx, key, jsonValue, expiration).Result()
	if err != nil {
		return false, fmt.Errorf("failed to set key %s: %w", key, err)
	}

	return result, nil
}

// ZAdd adds a member to a sorted set
func ZAdd(key string, score float64, member interface{}) error {
	jsonMember, err := json.Marshal(member)
	if err != nil {
		return fmt.Errorf("failed to marshal member: %w", err)
	}

	err = RedisClient.ZAdd(ctx, key, &redis.Z{
		Score:  score,
		Member: jsonMember,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to add to sorted set %s: %w", key, err)
	}

	return nil
}

// ZRange gets members from a sorted set
func ZRange(key string, start, stop int64) ([]string, error) {
	result, err := RedisClient.ZRange(ctx, key, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get range from sorted set %s: %w", key, err)
	}

	return result, nil
}

// ZRangeByScore gets members from a sorted set by score
func ZRangeByScore(key string, min, max string) ([]string, error) {
	result, err := RedisClient.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: min,
		Max: max,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get range by score from sorted set %s: %w", key, err)
	}

	return result, nil
}

// Increment atomically increments a key
func Increment(key string) (int64, error) {
	result, err := RedisClient.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}

	return result, nil
}

// Expire sets expiration for a key
func Expire(key string, expiration time.Duration) error {
	err := RedisClient.Expire(ctx, key, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to set expiration for key %s: %w", key, err)
	}

	return nil
}

// Pipeline creates a new Redis pipeline
func Pipeline() redis.Pipeliner {
	return RedisClient.Pipeline()
}

// Publish publishes a message to a channel
func Publish(channel string, message interface{}) error {
	jsonMessage, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = RedisClient.Publish(ctx, channel, jsonMessage).Err()
	if err != nil {
		return fmt.Errorf("failed to publish message to channel %s: %w", channel, err)
	}

	return nil
}

// Subscribe subscribes to Redis channels
func Subscribe(channels ...string) *redis.PubSub {
	return RedisClient.Subscribe(ctx, channels...)
}

// FlushDB flushes all keys in the current database
func FlushDB() error {
	err := RedisClient.FlushDB(ctx).Err()
	if err != nil {
		return fmt.Errorf("failed to flush database: %w", err)
	}

	return nil
}

// Close closes the Redis connection
func Close() error {
	if RedisClient != nil {
		return RedisClient.Close()
	}
	return nil
}

// HealthCheck checks if Redis is healthy
func HealthCheck() error {
	if RedisClient == nil {
		return fmt.Errorf("Redis client not initialized")
	}

	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("Redis ping failed: %w", err)
	}

	return nil
}

// Helper functions for common cache operations

// CacheOrderBookDepth caches order book depth data
func CacheOrderBookDepth(marketID string, depth interface{}) error {
	key := fmt.Sprintf(KeyOrderBookDepth, marketID)
	return Set(key, depth, ExpireOrderBookDepth)
}

// GetOrderBookDepth retrieves cached order book depth
func GetOrderBookDepth(marketID string, dest interface{}) error {
	key := fmt.Sprintf(KeyOrderBookDepth, marketID)
	return Get(key, dest)
}

// CacheMarketData caches market data
func CacheMarketData(marketID string, data interface{}) error {
	key := fmt.Sprintf(KeyMarketData, marketID)
	return Set(key, data, ExpireMarketData)
}

// GetMarketData retrieves cached market data
func GetMarketData(marketID string, dest interface{}) error {
	key := fmt.Sprintf(KeyMarketData, marketID)
	return Get(key, dest)
}

// CacheUserBalances caches user balances
func CacheUserBalances(userID uint, balances interface{}) error {
	key := fmt.Sprintf(KeyUserBalances, userID)
	return Set(key, balances, ExpireUserBalances)
}

// GetUserBalances retrieves cached user balances
func GetUserBalances(userID uint, dest interface{}) error {
	key := fmt.Sprintf(KeyUserBalances, userID)
	return Get(key, dest)
}

// InvalidateUserBalances removes cached user balances
func InvalidateUserBalances(userID uint) error {
	key := fmt.Sprintf(KeyUserBalances, userID)
	return Delete(key)
} 