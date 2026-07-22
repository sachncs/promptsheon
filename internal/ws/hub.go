// Package ws provides real-time log streaming via Server-Sent Events (SSE).
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LogEntry represents a log entry for streaming.
type LogEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Source    string         `json:"source"`
	Attrs     map[string]any `json:"attrs,omitempty"`
}

// Client represents an SSE client connection.
type Client struct {
	id     string
	send   chan []byte
	hub    *Hub
	filter LogFilter
}

// LogFilter defines filtering for log subscription. An empty
// slice (or nil) means "no filter on this dimension" — match
// anything. A non-empty slice means "match if the entry's level
// is in this set" (case-sensitive on the level name) or "match
// if the entry's source is in this set".
type LogFilter struct {
	Levels  []string `json:"levels,omitempty"`
	Sources []string `json:"sources,omitempty"`
}

// matches reports whether the entry passes every non-empty
// filter dimension. An empty Levels / Sources means the
// dimension is unconstrained.
func (f LogFilter) matches(entry *LogEntry) bool {
	if len(f.Levels) > 0 {
		matched := false
		for _, l := range f.Levels {
			if l == entry.Level {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(f.Sources) > 0 {
		matched := false
		for _, s := range f.Sources {
			if s == entry.Source {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// Hub manages SSE connections for log streaming.
type Hub struct {
	clients    map[string]*Client
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	logger     *slog.Logger
	nextID     int
	stop       chan struct{}
	done       chan struct{}
}

// NewHub creates a new SSE hub.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	defer close(h.done)
	for {
		select {
		case <-h.stop:
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.id] = client
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.id]; ok {
				delete(h.clients, client.id)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			var disconnected []string
			entry, ok := decodeLogEntry(message)
			h.mu.RLock()
			for id, client := range h.clients {
				if ok && !client.filter.matches(entry) {
					continue
				}
				select {
				case client.send <- message:
				default:
					disconnected = append(disconnected, id)
				}
			}
			h.mu.RUnlock()
			if len(disconnected) > 0 {
				h.mu.Lock()
				for _, id := range disconnected {
					if c, ok := h.clients[id]; ok {
						close(c.send)
						delete(h.clients, id)
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// Stop signals the hub goroutine to exit and waits for it. Safe to
// call more than once.
func (h *Hub) Stop() {
	select {
	case <-h.stop:
		return
	default:
		close(h.stop)
	}
	<-h.done
}

// BroadcastLog sends a log entry to all connected clients.
func (h *Hub) BroadcastLog(entry *LogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		h.logger.Error("failed to marshal log entry", "err", err)
		return
	}
	h.broadcast <- data
}

// splitCSV splits a comma-separated value into a slice of
// trimmed non-empty strings. Returns nil when the input is
// empty. Used to parse the ?level and ?source query parameters
// on /api/v1/logs/stream.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// decodeLogEntry reverses the JSON marshal in BroadcastLog so the
// broadcast loop can apply per-client filters without keeping a
// parallel channel of typed entries. Returns ok=false on parse
// error; the broadcast then delivers the raw message to every
// client (filters are best-effort).
func decodeLogEntry(data []byte) (*LogEntry, bool) {
	var e LogEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, false
	}
	return &e, true
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// HandleSSE handles Server-Sent Events connections. OBS-LOG-1:
// clients may filter the stream by ?level=info,warn&source=auth
// query parameters. Empty values mean "no filter on that
// dimension".
func (h *Hub) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	filter := LogFilter{
		Levels:  splitCSV(r.URL.Query().Get("level")),
		Sources: splitCSV(r.URL.Query().Get("source")),
	}

	h.mu.Lock()
	h.nextID++
	client := &Client{
		id:     fmt.Sprintf("client-%d", h.nextID),
		send:   make(chan []byte, 256),
		hub:    h,
		filter: filter,
	}
	h.mu.Unlock()

	h.register <- client

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Send initial connection event
	_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"client_id\":\"%s\"}\n\n", client.id)
	flusher.Flush()

	// Handle client disconnect
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			h.unregister <- client
			return
		case msg, ok := <-client.send:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

// LogStreamer wraps slog.Logger to stream logs via SSE.
type LogStreamer struct {
	Logger *slog.Logger
	Hub    *Hub
}

// NewLogStreamer creates a new log streamer.
func NewLogStreamer(logger *slog.Logger, hub *Hub) *LogStreamer {
	return &LogStreamer{
		Logger: logger,
		Hub:    hub,
	}
}

// StreamHandler returns an slog.Handler that broadcasts logs.
func (ls *LogStreamer) StreamHandler(next slog.Handler) slog.Handler {
	return &streamHandler{
		next:  next,
		hub:   ls.Hub,
		attrs: make([]slog.Attr, 0),
	}
}

type streamHandler struct {
	next  slog.Handler
	hub   *Hub
	attrs []slog.Attr
	group string
}

func (h *streamHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *streamHandler) Handle(_ context.Context, record slog.Record) error { //nolint:gocritic // must match slog.Handler interface
	// Forward to next handler
	if err := h.next.Handle(context.Background(), record); err != nil {
		return err
	}

	// Build log entry for streaming
	entry := &LogEntry{
		Timestamp: record.Time,
		Level:     record.Level.String(),
		Message:   record.Message,
		Source:    "promptsheon",
		Attrs:     make(map[string]any),
	}

	record.Attrs(func(a slog.Attr) bool {
		entry.Attrs[a.Key] = a.Value.Any()
		return true
	})

	for _, attr := range h.attrs {
		entry.Attrs[attr.Key] = attr.Value.Any()
	}

	// Broadcast to SSE clients
	h.hub.BroadcastLog(entry)

	return nil
}

func (h *streamHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handler := &streamHandler{
		next:  h.next,
		hub:   h.hub,
		attrs: append(h.attrs, attrs...),
		group: h.group,
	}
	return handler
}

func (h *streamHandler) WithGroup(name string) slog.Handler {
	handler := &streamHandler{
		next:  h.next.WithGroup(name),
		hub:   h.hub,
		attrs: h.attrs,
		group: name,
	}
	return handler
}
