package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"bixor-engine/pkg/models"
)

// WebSocketHub manages WebSocket connections
type WebSocketHub struct {
	// Registered clients
	clients map[*Client]bool
	
	// Inbound messages from clients
	broadcast chan []byte
	
	// Register requests from clients
	register chan *Client
	
	// Unregister requests from clients
	unregister chan *Client
	
	// Market subscriptions
	marketSubscriptions map[string]map[*Client]bool
	
	// User subscriptions
	userSubscriptions map[uint]map[*Client]bool
	
	// Mutex for thread-safe operations
	mu sync.RWMutex
}

// Client represents a WebSocket client
type Client struct {
	hub *WebSocketHub
	
	// WebSocket connection
	conn *websocket.Conn
	
	// Buffered channel of outbound messages
	send chan []byte
	
	// User information (nil if not authenticated)
	user *models.User
	
	// Client ID
	id string
	
	// Subscriptions
	subscriptions map[string]bool
	
	// Last seen timestamp
	lastSeen time.Time
}

// Message represents a WebSocket message
type Message struct {
	Type      string      `json:"type"`
	Channel   string      `json:"channel,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp"`
	ID        string      `json:"id,omitempty"`
}

// SubscriptionRequest represents a subscription request
type SubscriptionRequest struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Auth    string `json:"auth,omitempty"`
}

// Message types
const (
	MessageTypeSubscribe        = "subscribe"
	MessageTypeUnsubscribe      = "unsubscribe"
	MessageTypePing             = "ping"
	MessageTypePong             = "pong"
	MessageTypeError            = "error"
	MessageTypeOrderBookUpdate  = "orderbook_update"
	MessageTypeTradeUpdate      = "trade_update"
	MessageTypeOrderUpdate      = "order_update"
	MessageTypeBalanceUpdate    = "balance_update"
	MessageTypeMarketStatsUpdate = "market_stats_update"
)

// Channel types
const (
	ChannelOrderBook    = "orderbook"
	ChannelTrades       = "trades"
	ChannelMarketStats  = "market_stats"
	ChannelUserOrders   = "user_orders"
	ChannelUserBalances = "user_balances"
	ChannelUserTrades   = "user_trades"
)

// WebSocket connection settings
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// In production, implement proper origin checking
		return true
	},
}

// NewHub creates a new WebSocket hub
func NewHub() *WebSocketHub {
	return &WebSocketHub{
		clients:             make(map[*Client]bool),
		broadcast:           make(chan []byte),
		register:            make(chan *Client),
		unregister:          make(chan *Client),
		marketSubscriptions: make(map[string]map[*Client]bool),
		userSubscriptions:   make(map[uint]map[*Client]bool),
	}
}

// Run starts the WebSocket hub
func (h *WebSocketHub) Run(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		case message := <-h.broadcast:
			h.broadcastMessage(message)
		case <-ticker.C:
			h.pingClients()
		}
	}
}

// registerClient registers a new client
func (h *WebSocketHub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.clients[client] = true
	logrus.Infof("WebSocket client registered: %s", client.id)
	
	// Send welcome message
	welcome := Message{
		Type:      "welcome",
		Data:      map[string]interface{}{"client_id": client.id},
		Timestamp: time.Now().Unix(),
	}
	
	if data, err := json.Marshal(welcome); err == nil {
		select {
		case client.send <- data:
		default:
			close(client.send)
			delete(h.clients, client)
		}
	}
}

// unregisterClient unregisters a client
func (h *WebSocketHub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
		
		// Remove from market subscriptions
		for market, clients := range h.marketSubscriptions {
			if _, exists := clients[client]; exists {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.marketSubscriptions, market)
				}
			}
		}
		
		// Remove from user subscriptions
		if client.user != nil {
			if clients, exists := h.userSubscriptions[client.user.ID]; exists {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.userSubscriptions, client.user.ID)
				}
			}
		}
		
		logrus.Infof("WebSocket client unregistered: %s", client.id)
	}
}

// broadcastMessage broadcasts a message to all clients
func (h *WebSocketHub) broadcastMessage(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	for client := range h.clients {
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(h.clients, client)
		}
	}
}

// pingClients sends ping messages to all clients
func (h *WebSocketHub) pingClients() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	ping := Message{
		Type:      MessageTypePing,
		Timestamp: time.Now().Unix(),
	}
	
	if data, err := json.Marshal(ping); err == nil {
		for client := range h.clients {
			select {
			case client.send <- data:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

// SubscribeToMarket subscribes a client to market data
func (h *WebSocketHub) SubscribeToMarket(client *Client, marketID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.marketSubscriptions[marketID] == nil {
		h.marketSubscriptions[marketID] = make(map[*Client]bool)
	}
	h.marketSubscriptions[marketID][client] = true
	client.subscriptions[fmt.Sprintf("market:%s", marketID)] = true
	
	logrus.Infof("Client %s subscribed to market %s", client.id, marketID)
}

// UnsubscribeFromMarket unsubscribes a client from market data
func (h *WebSocketHub) UnsubscribeFromMarket(client *Client, marketID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if clients, exists := h.marketSubscriptions[marketID]; exists {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.marketSubscriptions, marketID)
		}
	}
	delete(client.subscriptions, fmt.Sprintf("market:%s", marketID))
	
	logrus.Infof("Client %s unsubscribed from market %s", client.id, marketID)
}

// SubscribeToUser subscribes a client to user-specific data
func (h *WebSocketHub) SubscribeToUser(client *Client, userID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.userSubscriptions[userID] == nil {
		h.userSubscriptions[userID] = make(map[*Client]bool)
	}
	h.userSubscriptions[userID][client] = true
	client.subscriptions[fmt.Sprintf("user:%d", userID)] = true
	
	logrus.Infof("Client %s subscribed to user %d", client.id, userID)
}

// BroadcastOrderBookUpdate broadcasts order book updates to subscribed clients
func (h *WebSocketHub) BroadcastOrderBookUpdate(marketID string, orderBook interface{}) {
	h.mu.RLock()
	clients := h.marketSubscriptions[marketID]
	h.mu.RUnlock()
	
	if len(clients) == 0 {
		return
	}
	
	message := Message{
		Type:      MessageTypeOrderBookUpdate,
		Channel:   fmt.Sprintf("%s.%s", ChannelOrderBook, marketID),
		Data:      orderBook,
		Timestamp: time.Now().Unix(),
	}
	
	if data, err := json.Marshal(message); err == nil {
		for client := range clients {
			select {
			case client.send <- data:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

// BroadcastTradeUpdate broadcasts trade updates to subscribed clients
func (h *WebSocketHub) BroadcastTradeUpdate(marketID string, trade interface{}) {
	h.mu.RLock()
	clients := h.marketSubscriptions[marketID]
	h.mu.RUnlock()
	
	if len(clients) == 0 {
		return
	}
	
	message := Message{
		Type:      MessageTypeTradeUpdate,
		Channel:   fmt.Sprintf("%s.%s", ChannelTrades, marketID),
		Data:      trade,
		Timestamp: time.Now().Unix(),
	}
	
	if data, err := json.Marshal(message); err == nil {
		for client := range clients {
			select {
			case client.send <- data:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

// BroadcastUserOrderUpdate broadcasts order updates to a specific user
func (h *WebSocketHub) BroadcastUserOrderUpdate(userID uint, order interface{}) {
	h.mu.RLock()
	clients := h.userSubscriptions[userID]
	h.mu.RUnlock()
	
	if len(clients) == 0 {
		return
	}
	
	message := Message{
		Type:      MessageTypeOrderUpdate,
		Channel:   ChannelUserOrders,
		Data:      order,
		Timestamp: time.Now().Unix(),
	}
	
	if data, err := json.Marshal(message); err == nil {
		for client := range clients {
			select {
			case client.send <- data:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

// BroadcastUserBalanceUpdate broadcasts balance updates to a specific user
func (h *WebSocketHub) BroadcastUserBalanceUpdate(userID uint, balances interface{}) {
	h.mu.RLock()
	clients := h.userSubscriptions[userID]
	h.mu.RUnlock()
	
	if len(clients) == 0 {
		return
	}
	
	message := Message{
		Type:      MessageTypeBalanceUpdate,
		Channel:   ChannelUserBalances,
		Data:      balances,
		Timestamp: time.Now().Unix(),
	}
	
	if data, err := json.Marshal(message); err == nil {
		for client := range clients {
			select {
			case client.send <- data:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

// HandleWebSocket handles WebSocket connections
func (h *WebSocketHub) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Error("WebSocket upgrade failed:", err)
		return
	}
	
	// Get user from context if authenticated
	var user *models.User
	if u, exists := c.Get("user"); exists {
		if userModel, ok := u.(*models.User); ok {
			user = userModel
		}
	}
	
	// Create client
	client := &Client{
		hub:           h,
		conn:          conn,
		send:          make(chan []byte, 256),
		user:          user,
		id:            fmt.Sprintf("%d", time.Now().UnixNano()),
		subscriptions: make(map[string]bool),
		lastSeen:      time.Now(),
	}
	
	// Register client
	h.register <- client
	
	// Start goroutines
	go client.writePump()
	go client.readPump()
}

// readPump handles reading messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		c.lastSeen = time.Now()
		return nil
	})
	
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logrus.Errorf("WebSocket error: %v", err)
			}
			break
		}
		
		c.handleMessage(message)
	}
}

// writePump handles writing messages to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			
			// Add queued messages to the current message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}
			
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage handles incoming messages from clients
func (c *Client) handleMessage(message []byte) {
	var req SubscriptionRequest
	if err := json.Unmarshal(message, &req); err != nil {
		c.sendError("Invalid message format")
		return
	}
	
	switch req.Type {
	case MessageTypeSubscribe:
		c.handleSubscribe(req)
	case MessageTypeUnsubscribe:
		c.handleUnsubscribe(req)
	case MessageTypePong:
		c.lastSeen = time.Now()
	default:
		c.sendError("Unknown message type")
	}
}

// handleSubscribe handles subscription requests
func (c *Client) handleSubscribe(req SubscriptionRequest) {
	// Parse channel
	switch {
	case req.Channel == ChannelOrderBook:
		// Subscribe to all market orderbooks
		c.hub.SubscribeToMarket(c, "all")
	case len(req.Channel) > len(ChannelOrderBook)+1 && req.Channel[:len(ChannelOrderBook)+1] == ChannelOrderBook+".":
		// Subscribe to specific market orderbook
		marketID := req.Channel[len(ChannelOrderBook)+1:]
		c.hub.SubscribeToMarket(c, marketID)
	case req.Channel == ChannelUserOrders || req.Channel == ChannelUserBalances:
		// Require authentication for user channels
		if c.user == nil {
			c.sendError("Authentication required for user channels")
			return
		}
		c.hub.SubscribeToUser(c, c.user.ID)
	default:
		c.sendError("Invalid channel")
		return
	}
	
	// Send subscription confirmation
	response := Message{
		Type:      "subscribed",
		Channel:   req.Channel,
		Timestamp: time.Now().Unix(),
	}
	
	if data, err := json.Marshal(response); err == nil {
		select {
		case c.send <- data:
		default:
			close(c.send)
		}
	}
}

// handleUnsubscribe handles unsubscription requests
func (c *Client) handleUnsubscribe(req SubscriptionRequest) {
	// Parse channel and unsubscribe
	switch {
	case req.Channel == ChannelOrderBook:
		c.hub.UnsubscribeFromMarket(c, "all")
	case len(req.Channel) > len(ChannelOrderBook)+1 && req.Channel[:len(ChannelOrderBook)+1] == ChannelOrderBook+".":
		marketID := req.Channel[len(ChannelOrderBook)+1:]
		c.hub.UnsubscribeFromMarket(c, marketID)
	}
	
	// Send unsubscription confirmation
	response := Message{
		Type:      "unsubscribed",
		Channel:   req.Channel,
		Timestamp: time.Now().Unix(),
	}
	
	if data, err := json.Marshal(response); err == nil {
		select {
		case c.send <- data:
		default:
			close(c.send)
		}
	}
}

// sendError sends an error message to the client
func (c *Client) sendError(message string) {
	errorMsg := Message{
		Type:      MessageTypeError,
		Data:      map[string]string{"error": message},
		Timestamp: time.Now().Unix(),
	}
	
	if data, err := json.Marshal(errorMsg); err == nil {
		select {
		case c.send <- data:
		default:
			close(c.send)
		}
	}
}

// GetStats returns WebSocket statistics
func (h *WebSocketHub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	stats := map[string]interface{}{
		"total_clients":        len(h.clients),
		"market_subscriptions": len(h.marketSubscriptions),
		"user_subscriptions":   len(h.userSubscriptions),
		"authenticated_clients": 0,
	}
	
	for client := range h.clients {
		if client.user != nil {
			stats["authenticated_clients"] = stats["authenticated_clients"].(int) + 1
		}
	}
	
	return stats
} 