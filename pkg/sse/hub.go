// Package sse provides Server-Sent Events support for GoPage.
package sse

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Event represents an SSE event.
type Event struct {
	ID      string `json:"id,omitempty"`
	Event   string `json:"event,omitempty"`
	Data    string `json:"data"`
	Channel string `json:"-"` // Internal: which channel to send to
}

// Client represents a connected SSE client.
type Client struct {
	ID       string
	Channels map[string]bool
	Events   chan *Event
	Done     chan struct{}
}

// Hub manages SSE connections and event distribution.
type Hub struct {
	clients    map[string]*Client
	channels   map[string]map[string]*Client // channel -> client_id -> client
	register   chan *Client
	unregister chan *Client
	broadcast  chan *Event
	mu         sync.RWMutex
	logger     *slog.Logger
}

// NewHub creates a new SSE hub.
func NewHub(logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	h := &Hub{
		clients:    make(map[string]*Client),
		channels:   make(map[string]map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *Event, 256),
		logger:     logger,
	}
	go h.run()
	return h
}

// run processes hub events.
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()
			h.logger.Debug("SSE client registered", "id", client.ID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				// Remove from all channels
				for channel := range client.Channels {
					if ch, ok := h.channels[channel]; ok {
						delete(ch, client.ID)
					}
				}
				delete(h.clients, client.ID)
				close(client.Events)
			}
			h.mu.Unlock()
			h.logger.Debug("SSE client unregistered", "id", client.ID)

		case event := <-h.broadcast:
			h.mu.RLock()
			if event.Channel == "" || event.Channel == "*" {
				// Broadcast to all clients
				for _, client := range h.clients {
					select {
					case client.Events <- event:
					default:
						// Client buffer full, skip
					}
				}
			} else {
				// Send to specific channel
				if ch, ok := h.channels[event.Channel]; ok {
					for _, client := range ch {
						select {
						case client.Events <- event:
						default:
						}
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Subscribe adds a client to a channel.
func (h *Hub) Subscribe(clientID, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client, ok := h.clients[clientID]
	if !ok {
		return
	}

	client.Channels[channel] = true

	if h.channels[channel] == nil {
		h.channels[channel] = make(map[string]*Client)
	}
	h.channels[channel][clientID] = client
}

// Unsubscribe removes a client from a channel.
func (h *Hub) Unsubscribe(clientID, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client, ok := h.clients[clientID]
	if !ok {
		return
	}

	delete(client.Channels, channel)
	if ch, ok := h.channels[channel]; ok {
		delete(ch, clientID)
	}
}

// Publish sends an event to a channel.
func (h *Hub) Publish(channel, eventType, data string) {
	h.broadcast <- &Event{
		ID:      fmt.Sprintf("%d", time.Now().UnixNano()),
		Event:   eventType,
		Data:    data,
		Channel: channel,
	}
}

// PublishJSON sends a JSON event to a channel.
func (h *Hub) PublishJSON(channel, eventType string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	h.Publish(channel, eventType, string(b))
	return nil
}

// Broadcast sends an event to all clients.
func (h *Hub) Broadcast(eventType, data string) {
	h.Publish("*", eventType, data)
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ChannelCount returns the number of clients in a channel.
func (h *Hub) ChannelCount(channel string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if ch, ok := h.channels[channel]; ok {
		return len(ch)
	}
	return 0
}

// ServeHTTP handles SSE connections.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client
	clientID := fmt.Sprintf("%d", time.Now().UnixNano())
	client := &Client{
		ID:       clientID,
		Channels: make(map[string]bool),
		Events:   make(chan *Event, 32),
		Done:     make(chan struct{}),
	}

	// Register client
	h.register <- client

	// Subscribe to channels from query params
	channels := r.URL.Query()["channel"]
	if len(channels) == 0 {
		channels = []string{"default"}
	}
	for _, ch := range channels {
		h.Subscribe(clientID, ch)
	}

	// Ensure cleanup on disconnect
	defer func() {
		h.unregister <- client
	}()

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"client_id\":\"%s\"}\n\n", clientID)
	flusher.Flush()

	// Keep-alive ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Event loop
	for {
		select {
		case <-r.Context().Done():
			return

		case event, ok := <-client.Events:
			if !ok {
				return
			}
			if event.ID != "" {
				fmt.Fprintf(w, "id: %s\n", event.ID)
			}
			if event.Event != "" {
				fmt.Fprintf(w, "event: %s\n", event.Event)
			}
			fmt.Fprintf(w, "data: %s\n\n", event.Data)
			flusher.Flush()

		case <-ticker.C:
			// Send keep-alive comment
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// Global hub instance
var globalHub *Hub
var hubOnce sync.Once

// GetHub returns the global SSE hub.
func GetHub() *Hub {
	hubOnce.Do(func() {
		globalHub = NewHub(nil)
	})
	return globalHub
}

// SetGlobalHub sets the global hub (call before GetHub).
func SetGlobalHub(h *Hub) {
	globalHub = h
}
