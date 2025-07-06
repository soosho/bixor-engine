package api

import (
	"net/http"

	"https://github.com/soosho/bixor-engine/internal/matching"
	"https://github.com/soosho/bixor-engine/pkg/config"
	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all API routes
func SetupRoutes(router *gin.Engine, engine *matching.MatchingEngine, cfg *config.Config) {
	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "bixor-engine",
			"version": "1.0.0",
		})
	})

	// API version group
	v1 := router.Group("/api/v1")
	{
		// Market endpoints
		markets := v1.Group("/markets")
		{
			markets.GET("", GetMarkets)
			markets.GET("/:marketId", GetMarket)
			markets.GET("/:marketId/orderbook", GetOrderBook)
			markets.GET("/:marketId/trades", GetTrades)
			markets.GET("/:marketId/stats", GetMarketStats)
			markets.GET("/:marketId/klines", GetKlines)
		}

		// Order endpoints
		orders := v1.Group("/orders")
		{
			orders.POST("", CreateOrder)
			orders.GET("", GetOrders)
			orders.GET("/:orderId", GetOrder)
			orders.DELETE("/:orderId", CancelOrder)
			orders.GET("/history", GetOrderHistory)
		}

		// User endpoints
		users := v1.Group("/users")
		{
			users.GET("/:userId/balances", GetUserBalances)
			users.GET("/:userId/orders", GetUserOrders)
			users.GET("/:userId/trades", GetUserTrades)
		}

		// Trading endpoints
		trading := v1.Group("/trading")
		{
			trading.POST("/orders", CreateOrder)
			trading.DELETE("/orders/:orderId", CancelOrder)
			trading.DELETE("/orders", CancelAllOrders)
		}

		// WebSocket endpoint for real-time data
		v1.GET("/ws", HandleWebSocket)
	}

	// Admin endpoints (if needed)
	admin := router.Group("/admin")
	{
		admin.GET("/health/database", CheckDatabaseHealth)
		admin.GET("/health/redis", CheckRedisHealth)
		admin.GET("/metrics", GetMetrics)
	}
} 