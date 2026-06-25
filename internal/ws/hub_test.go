package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	hub := NewHub(logger)

	if hub == nil {
		t.Fatal("expected hub to be non-nil")
	}
	if hub.clients == nil {
		t.Error("expected clients map to be initialized")
	}
	if hub.broadcast == nil {
		t.Error("expected broadcast channel to be initialized")
	}
}

func TestHubClientCount(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	hub := NewHub(logger)

	go hub.Run()

	if hub.ClientCount() != 0 {
		t.Errorf("ClientCount() = %d, want 0", hub.ClientCount())
	}
}

func TestHubRegisterClient(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	hub := NewHub(logger)

	go hub.Run()

	client := &Client{
		id:   "test-client",
		send: make(chan []byte, 256),
		hub:  hub,
	}

	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("ClientCount() = %d, want 1", hub.ClientCount())
	}
}

func TestHubUnregisterClient(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	hub := NewHub(logger)

	go hub.Run()

	client := &Client{
		id:   "test-client",
		send: make(chan []byte, 256),
		hub:  hub,
	}

	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	hub.unregister <- client
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("ClientCount() = %d, want 0", hub.ClientCount())
	}
}

func TestHubBroadcastLog(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	hub := NewHub(logger)

	go hub.Run()

	client := &Client{
		id:   "test-client",
		send: make(chan []byte, 256),
		hub:  hub,
	}

	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	entry := &LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "test message",
		Source:    "test",
	}

	hub.BroadcastLog(entry)

	select {
	case msg := <-client.send:
		if len(msg) == 0 {
			t.Error("expected non-empty message")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for broadcast message")
	}
}

func TestHubBroadcastToMultipleClients(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	hub := NewHub(logger)

	go hub.Run()

	client1 := &Client{id: "c1", send: make(chan []byte, 256), hub: hub}
	client2 := &Client{id: "c2", send: make(chan []byte, 256), hub: hub}

	hub.register <- client1
	hub.register <- client2
	time.Sleep(10 * time.Millisecond)

	entry := &LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "broadcast test",
		Source:    "test",
	}

	hub.BroadcastLog(entry)

	// Both clients should receive the message
	for _, c := range []*Client{client1, client2} {
		select {
		case msg := <-c.send:
			if len(msg) == 0 {
				t.Errorf("client %s: expected non-empty message", c.id)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("client %s: timeout waiting for broadcast message", c.id)
		}
	}
}

func TestLogEntryJSON(t *testing.T) {
	entry := &LogEntry{
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Level:     "info",
		Message:   "test message",
		Source:    "test-source",
		Attrs:     map[string]any{"key": "value"},
	}

	if entry.Level != "info" {
		t.Errorf("Level = %q, want %q", entry.Level, "info")
	}
	if entry.Message != "test message" {
		t.Errorf("Message = %q, want %q", entry.Message, "test message")
	}
	if entry.Source != "test-source" {
		t.Errorf("Source = %q, want %q", entry.Source, "test-source")
	}
}

func TestLogFilter(t *testing.T) {
	filter := LogFilter{
		Levels:  []string{"info", "warn"},
		Sources: []string{"api", "llm"},
	}

	if len(filter.Levels) != 2 {
		t.Errorf("Levels len = %d, want 2", len(filter.Levels))
	}
	if len(filter.Sources) != 2 {
		t.Errorf("Sources len = %d, want 2", len(filter.Sources))
	}
}

func TestNewLogStreamer(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	hub := NewHub(logger)
	streamer := NewLogStreamer(logger, hub)

	if streamer == nil {
		t.Fatal("expected streamer to be non-nil")
	}
	if streamer.logger != logger {
		t.Error("expected logger to be set")
	}
	if streamer.hub != hub {
		t.Error("expected hub to be set")
	}
}

// TestBroadcastLogDeliversToClient runs the hub, registers a
// client, broadcasts a log entry, and confirms the client
// receives the JSON-encoded message on its send channel.
func TestBroadcastLogDeliversToClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{
		id:   "test-broadcast",
		send: make(chan []byte, 16),
		hub:  hub,
	}
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastLog(&LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "hello",
		Source:    "test",
	})

	select {
	case msg := <-client.send:
		var entry LogEntry
		if err := json.Unmarshal(msg, &entry); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if entry.Message != "hello" {
			t.Errorf("expected message 'hello', got %q", entry.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast")
	}
}

// TestHandleSSEWritesEventStreamHeaders confirms the
// content-type and the initial 'connected' event make it to
// the response. The handler blocks on the request context,
// so we run it in a goroutine and cancel the request after
// capturing the buffered output.
func TestHandleSSEWritesEventStreamHeaders(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(logger)
	go hub.Run()

	rec := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/logs/stream", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		hub.HandleSSE(rec, req)
	}()

	// Give the handler a moment to write the headers and the
	// initial 'connected' event.
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type: got %q, want text/event-stream", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control: got %q, want no-cache", got)
	}
	if !strings.Contains(rec.Body.String(), "event: connected") {
		t.Errorf("expected 'event: connected' in body, got %q", rec.Body.String())
	}
}

// TestHandleSSERefusesNonFlusher confirms that
// non-Flusher ResponseWriters get a 500 error rather than
// hanging the handler.
func TestHandleSSERefusesNonFlusher(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(logger)
	go hub.Run()

	// http.ResponseWriter that does NOT implement http.Flusher.
	rw := &nonFlusher{}
	req := httptest.NewRequest("GET", "/logs/stream", nil)
	hub.HandleSSE(rw, req)
	if rw.status != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rw.status)
	}
}

type nonFlusher struct {
	header http.Header
	status int
	body   strings.Builder
}

func (n *nonFlusher) Header() http.Header {
	if n.header == nil {
		n.header = make(http.Header)
	}
	return n.header
}
func (n *nonFlusher) Write(b []byte) (int, error) { return n.body.Write(b) }
func (n *nonFlusher) WriteHeader(s int)          { n.status = s }

// TestStreamHandlerForwardsAndBroadcasts builds a chain of
// two slog handlers, with the inner one being the streaming
// handler. A log line on the outer chain should reach the
// inner hub and (since the hub is running) be broadcast to
// any connected clients.
func TestStreamHandlerForwardsAndBroadcasts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{
		id:   "streamer",
		send: make(chan []byte, 16),
		hub:  hub,
	}
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	streamer := NewLogStreamer(logger, hub)
	handler := streamer.StreamHandler(slog.NewTextHandler(io.Discard, nil))

	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "streamed", 0)
	if err := handler.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	select {
	case msg := <-client.send:
		var entry LogEntry
		if err := json.Unmarshal(msg, &entry); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if entry.Message != "streamed" {
			t.Errorf("expected message 'streamed', got %q", entry.Message)
		}
		if entry.Level != "INFO" {
			t.Errorf("expected level INFO, got %q", entry.Level)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for streamed log")
	}
}

func TestStreamHandlerEnabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(logger)
	streamer := NewLogStreamer(logger, hub)
	handler := streamer.StreamHandler(slog.NewTextHandler(io.Discard, nil))
	if !handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected Enabled to return true")
	}
	if !handler.Enabled(context.Background(), slog.LevelError) {
		t.Error("expected Enabled to return true for any level")
	}
}

func TestStreamHandlerWithAttrsAndGroup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(logger)
	streamer := NewLogStreamer(logger, hub)
	handler := streamer.StreamHandler(slog.NewTextHandler(io.Discard, nil))

	// WithAttrs and WithGroup should return a new handler
	// (not the same one) so the outer slog can build
	// attribute stacks.
	attrs := []slog.Attr{slog.String("k", "v")}
	withAttrs := handler.WithAttrs(attrs)
	if withAttrs == nil {
		t.Fatal("WithAttrs returned nil")
	}
	withGroup := handler.WithGroup("grp")
	if withGroup == nil {
		t.Fatal("WithGroup returned nil")
	}
}
