package api

import (
	"net/http"
	"strconv"
	"time"

	"https://github.com/soosho/bixor-engine/pkg/cache"
	"https://github.com/soosho/bixor-engine/pkg/database"
	"https://github.com/soosho/bixor-engine/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
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
			"klines":    []interface{}{},
		},
	})
}

// Order Handlers

// CreateOrder creates a new trading order
func CreateOrder(c *gin.Context) {
	var req struct {
		MarketID string `json:"market_id" binding:"required"`
		Side     int8   `json:"side" binding:"required"`
		Type     string `json:"type" binding:"required"`
		Price    string `json:"price"`
		Size     string `json:"size" binding:"required"`
		UserID   uint   `json:"user_id" binding:"required"`
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

	// Create order
	order := models.Order{
		ID:       generateOrderID(),
		UserID:   req.UserID,
		MarketID: req.MarketID,
		Side:     models.OrderSide(req.Side),
		Type:     models.OrderType(req.Type),
		Status:   models.OrderStatusPending,
		Price:    models.DecimalFromString(req.Price),
		Size:     models.DecimalFromString(req.Size),
	}

	if err := database.GetDB().Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    order,
	})
}

// GetOrders returns user's orders
func GetOrders(c *gin.Context) {
	userIDStr := c.Query("user_id")
	marketID := c.Query("market_id")
	status := c.Query("status")
	
	query := database.GetDB().Model(&models.Order{})
	
	if userIDStr != "" {
		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
			return
		}
		query = query.Where("user_id = ?", userID)
	}
	
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
	orderID := c.Param("orderId")
	
	var order models.Order
	if err := database.GetDB().Where("id = ?", orderID).First(&order).Error; err != nil {
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
	orderID := c.Param("orderId")
	
	var order models.Order
	if err := database.GetDB().Where("id = ?", orderID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	if order.Status != models.OrderStatusOpen && order.Status != models.OrderStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order cannot be cancelled"})
		return
	}

	now := time.Now()
	order.Status = models.OrderStatusCancelled
	order.CancelledAt = &now

	if err := database.GetDB().Save(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    order,
	})
}

// CancelAllOrders cancels all open orders for a user
func CancelAllOrders(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	now := time.Now()
	result := database.GetDB().Model(&models.Order{}).
		Where("user_id = ? AND status IN (?)", userID, []string{"open", "pending"}).
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
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	var orders []models.Order
	if err := database.GetDB().Where("user_id = ?", userID).
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
	userIDStr := c.Param("userId")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var balances []models.Balance
	if err := database.GetDB().Where("user_id = ?", userID).Find(&balances).Error; err != nil {
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
	userIDStr := c.Param("userId")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var orders []models.Order
	if err := database.GetDB().Where("user_id = ?", userID).
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
	userIDStr := c.Param("userId")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var trades []models.Trade
	if err := database.GetDB().Where("taker_user_id = ? OR maker_user_id = ?", userID, userID).
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
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Error("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	// Handle WebSocket connection
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			logrus.Error("WebSocket read error:", err)
			break
		}

		// Echo message back (implement real-time data streaming here)
		if err := conn.WriteMessage(messageType, message); err != nil {
			logrus.Error("WebSocket write error:", err)
			break
		}
	}
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