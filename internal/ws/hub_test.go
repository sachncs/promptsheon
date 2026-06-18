package ws

import (
	"log/slog"
	"os"
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
