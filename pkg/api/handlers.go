package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"bixor-engine/internal/matching"
	"bixor-engine/pkg/cache"
	"bixor-engine/pkg/database"
	"bixor-engine/pkg/middleware"
	"bixor-engine/pkg/models"
	"bixor-engine/pkg/websocket"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// TradingHandlers contains trading-related handlers with matching engine
type TradingHandlers struct {
	engine *matching.MatchingEngine
	hub    *websocket.WebSocketHub
}

// NewTradingHandlers creates new trading handlers
func NewTradingHandlers(engine *matching.MatchingEngine, hub *websocket.WebSocketHub) *TradingHandlers {
	return &TradingHandlers{
		engine: engine,
		hub:    hub,
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

var globalWSHub *websocket.WebSocketHub
var globalTradingHandlers *TradingHandlers

// GetWebSocketHub returns the global WebSocket hub instance
func GetWebSocketHub() *websocket.WebSocketHub {
	if globalWSHub == nil {
		globalWSHub = websocket.NewHub()
	}
	return globalWSHub
}

// GetTradingHandlers returns the global trading handlers instance
func GetTradingHandlers() *TradingHandlers {
	return globalTradingHandlers
}

// SetTradingHandlers sets the global trading handlers instance
func SetTradingHandlers(handlers *TradingHandlers) {
	globalTradingHandlers = handlers
}

// Market Handlers

// GetMarkets returns all available trading markets
func GetMarkets(c *gin.Context) {
	var markets []models.Market
	
	if err := database.GetDB().Where("is_active = ?", true).Find(&markets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch markets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    markets,
	})
}

// GetMarket returns a specific market
func GetMarket(c *gin.Context) {
	marketID := c.Param("marketId")
	
	var market models.Market
	if err := database.GetDB().Where("id = ?", marketID).First(&market).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    market,
	})
}

// GetOrderBook returns order book depth for a market
func GetOrderBook(c *gin.Context) {
	marketID := c.Param("marketId")
	limitStr := c.DefaultQuery("limit", "50")
	
	limit, err := strconv.ParseUint(limitStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter"})
		return
	}

	// Try to get from cache first
	var depth interface{}
	if err := cache.GetOrderBookDepth(marketID, &depth); err == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    depth,
			"limit":   limit, // Include limit in response
		})
		return
	}

	// If not in cache, return empty order book
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"market_id": marketID,
			"bids":      []interface{}{},
			"asks":      []interface{}{},
			"timestamp": time.Now().Unix(),
			"limit":     limit, // Include limit in response
		},
	})
}

// GetTrades returns recent trades for a market
func GetTrades(c *gin.Context) {
	marketID := c.Param("marketId")
	limitStr := c.DefaultQuery("limit", "100")
	
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter"})
		return
	}

	var trades []models.Trade
	if err := database.GetDB().Where("market_id = ?", marketID).
		Order("created_at DESC").
		Limit(limit).
		Find(&trades).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trades"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    trades,
	})
}

// GetMarketStats returns market statistics
func GetMarketStats(c *gin.Context) {
	marketID := c.Param("marketId")
	
	var marketData models.MarketData
	if err := database.GetDB().Where("market_id = ?", marketID).First(&marketData).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market data not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    marketData,
	})
}

// GetKlines returns candlestick data
func GetKlines(c *gin.Context) {
	marketID := c.Param("marketId")
	interval := c.DefaultQuery("interval", "1m")
	limitStr := c.DefaultQuery("limit", "100")
	
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter"})
		return
	}

	// For now, return empty klines (would implement OHLCV logic here)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"market_id": marketID,
			"interval":  interval,
			"limit":     limit,
			"klines":    []interface{}{},
		},
	})
}

// Order Handlers

// CreateOrder creates a new trading order
func CreateOrder(c *gin.Context) {
	// Get authenticated user from context
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req struct {
		MarketID string `json:"market_id" binding:"required"`
		Side     int8   `json:"side" binding:"required"`
		Type     string `json:"type" binding:"required"`
		Price    string `json:"price"`
		Size     string `json:"size" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate market exists
	var market models.Market
	if err := database.GetDB().Where("id = ? AND is_active = ?", req.MarketID, true).First(&market).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid market"})
		return
	}

	// Validate order side and type
	if req.Side != 1 && req.Side != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order side (1=buy, 2=sell)"})
		return
	}

	// Validate order price for limit orders
	price := models.DecimalFromString(req.Price)
	size := models.DecimalFromString(req.Size)
	
	if req.Type == "limit" && price.IsZero() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Price required for limit orders"})
		return
	}

	if size.IsZero() || size.IsNegative() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order size"})
		return
	}

	// Check user balance before creating order
	if req.Side == 1 { // Buy order - check quote asset balance
		var balance models.Balance
		requiredAmount := price.Mul(size)
		if err := database.GetDB().Where("user_id = ? AND asset = ?", user.ID, market.QuoteAsset).First(&balance).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient balance"})
			return
		}
		if balance.Available.LessThan(requiredAmount) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient balance"})
			return
		}
	} else { // Sell order - check base asset balance
		var balance models.Balance
		if err := database.GetDB().Where("user_id = ? AND asset = ?", user.ID, market.BaseAsset).First(&balance).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient balance"})
			return
		}
		if balance.Available.LessThan(size) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient balance"})
			return
		}
	}

	// Create order using authenticated user ID
	orderID := generateOrderID()
	order := models.Order{
		ID:       orderID,
		UserID:   user.ID,
		MarketID: req.MarketID,
		Side:     models.OrderSide(req.Side),
		Type:     models.OrderType(req.Type),
		Status:   models.OrderStatusPending,
		Price:    price,
		Size:     size,
	}

	// Save order to database first
	if err := database.GetDB().Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
		return
	}

	// Get trading handlers
	tradingHandlers := GetTradingHandlers()
	if tradingHandlers != nil && tradingHandlers.engine != nil {
		// Convert to matching engine order format
		matchingOrder := &matching.Order{
			ID:        orderID,
			MarketID:  req.MarketID,
			Side:      matching.Side(req.Side),
			Price:     price,
			Size:      size,
			Type:      matching.OrderType(req.Type),
			UserID:    int64(user.ID),
			CreatedAt: time.Now(),
		}

		// Submit order to matching engine
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := tradingHandlers.engine.AddOrder(ctx, matchingOrder); err != nil {
			// If matching engine fails, mark order as failed but don't delete it
			order.Status = models.OrderStatusFailed
			database.GetDB().Save(&order)
			
			logrus.Errorf("Failed to submit order to matching engine: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit order to matching engine"})
			return
		}

		// Update order status to open
		order.Status = models.OrderStatusOpen
		database.GetDB().Save(&order)

		// Broadcast order update to user via WebSocket
		if tradingHandlers.hub != nil {
			tradingHandlers.hub.BroadcastUserOrderUpdate(user.ID, order)
		}
	} else {
		// No matching engine available, keep order as pending
		logrus.Warn("No matching engine available, order remains pending")
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    order,
	})
}

// GetOrders returns user's orders
func GetOrders(c *gin.Context) {
	// Get authenticated user from context
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	marketID := c.Query("market_id")
	status := c.Query("status")
	
	// Always filter by authenticated user ID
	query := database.GetDB().Model(&models.Order{}).Where("user_id = ?", user.ID)
	
	if marketID != "" {
		query = query.Where("market_id = ?", marketID)
	}
	
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var orders []models.Order
	if err := query.Order("created_at DESC").Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    orders,
	})
}

// GetOrder returns a specific order
func GetOrder(c *gin.Context) {
	// Get authenticated user from context
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	orderID := c.Param("orderId")
	
	var order models.Order
	if err := database.GetDB().Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    order,
	})
}

// CancelOrder cancels an order
func CancelOrder(c *gin.Context) {
	// Get authenticated user from context
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	orderID := c.Param("orderId")
	
	var order models.Order
	if err := database.GetDB().Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	if order.Status != models.OrderStatusOpen && order.Status != models.OrderStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order cannot be cancelled"})
		return
	}

	// Get trading handlers and cancel from matching engine first
	tradingHandlers := GetTradingHandlers()
	if tradingHandlers != nil && tradingHandlers.engine != nil && order.Status == models.OrderStatusOpen {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := tradingHandlers.engine.CancelOrder(ctx, order.MarketID, orderID); err != nil {
			logrus.Errorf("Failed to cancel order in matching engine: %v", err)
			// Continue with database cancellation even if matching engine fails
		}
	}

	// Update order status in database
	now := time.Now()
	order.Status = models.OrderStatusCancelled
	order.CancelledAt = &now

	if err := database.GetDB().Save(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order"})
		return
	}

	// Broadcast order update to user via WebSocket
	if tradingHandlers != nil && tradingHandlers.hub != nil {
		tradingHandlers.hub.BroadcastUserOrderUpdate(user.ID, order)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    order,
	})
}

// CancelAllOrders cancels all open orders for a user
func CancelAllOrders(c *gin.Context) {
	// Get authenticated user from context
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	now := time.Now()
	result := database.GetDB().Model(&models.Order{}).
		Where("user_id = ? AND status IN (?)", user.ID, []string{"open", "pending"}).
		Updates(map[string]interface{}{
			"status":       models.OrderStatusCancelled,
			"cancelled_at": now,
		})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel orders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Orders cancelled",
		"count":   result.RowsAffected,
	})
}

// GetOrderHistory returns order history
func GetOrderHistory(c *gin.Context) {
	// Get authenticated user from context
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var orders []models.Order
	if err := database.GetDB().Where("user_id = ?", user.ID).
		Order("created_at DESC").
		Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch order history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    orders,
	})
}

// User Handlers

// GetUserBalances returns user's balances
func GetUserBalances(c *gin.Context) {
	// Get authenticated user from context
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var balances []models.Balance
	if err := database.GetDB().Where("user_id = ?", user.ID).Find(&balances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch balances"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    balances,
	})
}

// GetUserOrders returns user's orders
func GetUserOrders(c *gin.Context) {
	// Get authenticated user from context
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var orders []models.Order
	if err := database.GetDB().Where("user_id = ?", user.ID).
		Order("created_at DESC").
		Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    orders,
	})
}

// GetUserTrades returns user's trades
func GetUserTrades(c *gin.Context) {
	// Get authenticated user from context
	user, exists := middleware.GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var trades []models.Trade
	if err := database.GetDB().Where("taker_user_id = ? OR maker_user_id = ?", user.ID, user.ID).
		Order("created_at DESC").
		Find(&trades).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trades"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    trades,
	})
}

// WebSocket Handler
func HandleWebSocket(c *gin.Context) {
	// Get the WebSocket hub
	hub := GetWebSocketHub()
	hub.HandleWebSocket(c)
}

// Admin Handlers

// CheckDatabaseHealth checks database connectivity
func CheckDatabaseHealth(c *gin.Context) {
	if err := database.HealthCheck(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// CheckRedisHealth checks Redis connectivity
func CheckRedisHealth(c *gin.Context) {
	if err := cache.HealthCheck(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// GetMetrics returns system metrics
func GetMetrics(c *gin.Context) {
	// Get database stats
	var userCount, orderCount, tradeCount int64
	database.GetDB().Model(&models.User{}).Count(&userCount)
	database.GetDB().Model(&models.Order{}).Count(&orderCount)
	database.GetDB().Model(&models.Trade{}).Count(&tradeCount)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"users":  userCount,
			"orders": orderCount,
			"trades": tradeCount,
			"uptime": time.Now().Format(time.RFC3339),
		},
	})
}

// Helper functions

func generateOrderID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
} 