package events

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketHub manages WebSocket client connections
type WebSocketHub struct {
	// Registered clients
	clients map[*Client]bool

	// Subscribe requests
	subscribe chan *Client

	// Unsubscribe requests
	unsubscribe chan *Client

	// Inbound messages
	broadcast chan []byte

	// Global event bus
	eventBus *EventBus

	// Connection configuration
	upgrader websocket.Upgrader

	// Client mutex
	mu sync.RWMutex
}

// Client represents a WebSocket client connection
type Client struct {
	// Hub reference
	hub *WebSocketHub

	// WebSocket connection
	conn *websocket.Conn

	// Buffered channel for outbound messages
	send chan []byte

	// Subscribed topics
	subscriptions map[string]bool

	// Client mutex
	mu sync.Mutex
}

// WSMessage represents a message from WebSocket client
type WSMessage struct {
	Action  string   `json:"action"`  // "subscribe", "unsubscribe", "ping"
	Topics  []string `json:"topics"`  // Topics to subscribe/unsubscribe
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub(eventBus *EventBus) *WebSocketHub {
	if eventBus == nil {
		eventBus = GetBus()
	}

	return &WebSocketHub{
		clients:     make(map[*Client]bool),
		subscribe:   make(chan *Client),
		unsubscribe: make(chan *Client),
		broadcast:   make(chan []byte),
		eventBus:    eventBus,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// In production, implement proper origin checking
				return true
			},
		},
	}
}

// Run starts the hub's main loop
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.subscribe:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected (total: %d)", len(h.clients))

		case client := <-h.unsubscribe:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected (total: %d)", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client buffer full, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// HandleWebSocket upgrades HTTP connection to WebSocket
func (h *WebSocketHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:           h,
		conn:          conn,
		send:          make(chan []byte, 256),
		subscriptions: make(map[string]bool),
	}

	h.subscribe <- client

	// Start client goroutines
	go client.writePump()
	go client.readPump()
}

// ServeWS is an http.HandlerFunc for WebSocket connections
func (h *WebSocketHub) ServeWS(w http.ResponseWriter, r *http.Request) {
	h.HandleWebSocket(w, r)
}

// BroadcastEvent sends an event to all connected clients
func (h *WebSocketHub) BroadcastEvent(event *Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Error marshaling event: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		// Check if client is subscribed to this topic
		client.mu.Lock()
		subscribed := client.subscriptions[event.Topic] || client.subscriptions[TopicAll]
		client.mu.Unlock()

		if subscribed {
			select {
			case client.send <- data:
			default:
				// Buffer full
			}
		}
	}
}

// BroadcastEventToTopic sends an event to clients subscribed to a specific topic
func (h *WebSocketHub) BroadcastEventToTopic(topic string, event *Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Error marshaling event: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		client.mu.Lock()
		subscribed := client.subscriptions[topic] || client.subscriptions[TopicAll]
		client.mu.Unlock()

		if subscribed {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// ClientCount returns the number of connected clients
func (h *WebSocketHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unsubscribe <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.handleMessage(message)
	}
}

// handleMessage processes incoming WebSocket messages
func (c *Client) handleMessage(message []byte) {
	var msg WSMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Invalid WebSocket message: %v", err)
		return
	}

	switch msg.Action {
	case "subscribe":
		for _, topic := range msg.Topics {
			c.mu.Lock()
			c.subscriptions[topic] = true
			c.mu.Unlock()
			log.Printf("Client subscribed to topic: %s", topic)
		}

		// Send confirmation
		c.sendJSON(map[string]interface{}{
			"type":    "subscribed",
			"topics":  msg.Topics,
			"success": true,
		})

	case "unsubscribe":
		for _, topic := range msg.Topics {
			c.mu.Lock()
			delete(c.subscriptions, topic)
			c.mu.Unlock()
			log.Printf("Client unsubscribed from topic: %s", topic)
		}

		c.sendJSON(map[string]interface{}{
			"type":    "unsubscribed",
			"topics":  msg.Topics,
			"success": true,
		})

	case "ping":
		c.sendJSON(map[string]string{"type": "pong"})

	default:
		log.Printf("Unknown WebSocket action: %s", msg.Action)
	}
}

// writePump pumps messages from the send channel to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Channel was closed
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Batch pending messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// sendJSON sends a JSON message to the client
func (c *Client) sendJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		return
	}

	select {
	case c.send <- data:
	default:
	}
}
