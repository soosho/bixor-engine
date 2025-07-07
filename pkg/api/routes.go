package api

import (
	"net/http"
	"time"

	"bixor-engine/internal/matching"
	"bixor-engine/pkg/auth"
	"bixor-engine/pkg/cache"
	"bixor-engine/pkg/config"
	"bixor-engine/pkg/database"
	"bixor-engine/pkg/middleware"
	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all API routes
func SetupRoutes(router *gin.Engine, engine *matching.MatchingEngine, cfg *config.Config, redisCache *cache.RedisCache) {
	// Initialize authentication services
	jwtService := auth.NewJWTService(
		cfg.Auth.JWTSecret,
		time.Duration(cfg.Auth.AccessTokenTTL)*time.Second,
		time.Duration(cfg.Auth.RefreshTokenTTL)*time.Second,
	)
	totpService := auth.NewTOTPService("Bixor Exchange")
	
	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtService, database.GetDB())
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(redisCache, database.GetDB())
	sessionMiddleware := middleware.NewSessionMiddleware(database.GetDB())
	
	// Initialize auth handlers
	authHandlers := NewAuthHandlers(
		database.GetDB(),
		jwtService,
		totpService,
		authMiddleware,
		sessionMiddleware,
	)
	
	// Initialize trading handlers  
	hub := GetWebSocketHub()
	tradingHandlers := NewTradingHandlers(engine, hub)
	SetTradingHandlers(tradingHandlers)

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "bixor-engine",
			"version": "1.0.0",
		})
	})

	// Setup Swagger documentation
	setupSwagger(router)

	// Apply global rate limiting to all routes
	router.Use(rateLimitMiddleware.IPRateLimit(middleware.DefaultRateLimit))

	// API version group
	v1 := router.Group("/api/v1")
	{
		// Public authentication endpoints (no auth required)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandlers.Register)
			auth.POST("/login", authHandlers.Login)
			auth.POST("/refresh", authHandlers.RefreshToken)
		}

		// Protected authentication endpoints (auth required)
		authProtected := v1.Group("/auth")
		authProtected.Use(authMiddleware.JWTAuth())
		{
			authProtected.POST("/logout", authHandlers.Logout)
			authProtected.GET("/profile", authHandlers.GetProfile)
			authProtected.POST("/2fa/enable", authHandlers.Enable2FA)
			authProtected.POST("/2fa/verify", authHandlers.Verify2FA)
			authProtected.POST("/2fa/disable", authHandlers.Disable2FA)
			authProtected.POST("/api-keys", authHandlers.CreateAPIKey)
			authProtected.GET("/api-keys", authHandlers.ListAPIKeys)
			authProtected.DELETE("/api-keys/:key_id", authHandlers.RevokeAPIKey)
		}
		// Public market endpoints (higher rate limits)
		markets := v1.Group("/markets")
		markets.Use(rateLimitMiddleware.PublicRateLimit())
		{
			markets.GET("", GetMarkets)
			markets.GET("/:marketId", GetMarket)
			markets.GET("/:marketId/orderbook", GetOrderBook)
			markets.GET("/:marketId/trades", GetTrades)
			markets.GET("/:marketId/stats", GetMarketStats)
			markets.GET("/:marketId/klines", GetKlines)
		}

		// Order endpoints (require authentication and verification)
		orders := v1.Group("/orders")
		orders.Use(authMiddleware.JWTAuth())
		orders.Use(middleware.RequireVerified())
		orders.Use(rateLimitMiddleware.TradingRateLimit())
		{
			orders.POST("", CreateOrder)
			orders.GET("", GetOrders)
			orders.GET("/:orderId", GetOrder)
			orders.DELETE("/:orderId", CancelOrder)
			orders.DELETE("", CancelAllOrders)
			orders.GET("/history", GetOrderHistory)
		}

		// User endpoints (require authentication and verified accounts)
		users := v1.Group("/users")
		users.Use(authMiddleware.JWTAuth())
		users.Use(middleware.RequireVerified())
		{
			users.GET("/me/balances", GetUserBalances)
			users.GET("/me/orders", GetUserOrders)
			users.GET("/me/trades", GetUserTrades)
		}

		// WebSocket endpoint for real-time data (requires authentication)
		ws := v1.Group("/ws")
		ws.Use(authMiddleware.OptionalAuth()) // Allow both authenticated and anonymous connections
		{
			ws.GET("", HandleWebSocket)
		}
	}

	// Admin endpoints (require admin authentication)
	admin := router.Group("/admin")
	admin.Use(authMiddleware.JWTAuth())
	admin.Use(middleware.RequireAdmin())
	{
		admin.GET("/health/database", CheckDatabaseHealth)
		admin.GET("/health/redis", CheckRedisHealth)
		admin.GET("/metrics", GetMetrics)
		// TODO: Implement these admin handlers
		// admin.GET("/users", GetAllUsers)
		// admin.POST("/users/:userId/verify", VerifyUser)
		// admin.POST("/users/:userId/activate", ActivateUser)
		// admin.POST("/users/:userId/deactivate", DeactivateUser)
		// admin.GET("/rate-limit/stats", GetRateLimitStats)
		// admin.GET("/login-attempts", GetLoginAttempts)
		// admin.GET("/sessions", GetAllSessions)
	}
} 