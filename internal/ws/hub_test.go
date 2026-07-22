package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
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
		Level:   "info",
		Message: "test message",
		Source:  "test-source",
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
	if streamer.Logger != logger {
		t.Error("expected logger to be set")
	}
	if streamer.Hub != hub {
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
func (n *nonFlusher) WriteHeader(s int)           { n.status = s }

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

func TestRun_DisconnectsSlowClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{
		id:   "slow",
		send: make(chan []byte, 1),
		hub:  hub,
	}
	hub.register <- client
	time.Sleep(5 * time.Millisecond)

	hub.BroadcastLog(&LogEntry{Message: "a"})
	hub.BroadcastLog(&LogEntry{Message: "b"})
	time.Sleep(20 * time.Millisecond)

	if got := hub.ClientCount(); got != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", got)
	}
}

func TestBroadcastLog_MarshalError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	hub := NewHub(logger)

	entry := &LogEntry{
		Attrs: map[string]any{"bad": make(chan int)},
	}
	hub.BroadcastLog(entry)

	if !strings.Contains(buf.String(), "failed to marshal") {
		t.Errorf("expected marshal error log, got %q", buf.String())
	}
}

func TestHandleSSE_DeliversLogEvent(t *testing.T) {
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
	time.Sleep(20 * time.Millisecond)

	hub.BroadcastLog(&LogEntry{Message: "sse-delivery-test"})
	time.Sleep(20 * time.Millisecond)

	cancel()
	<-done

	if !strings.Contains(rec.Body.String(), "sse-delivery-test") {
		t.Errorf("expected log event in body, got %q", rec.Body.String())
	}
}

type errHandler struct{}

func (e errHandler) Enabled(context.Context, slog.Level) bool  { return true }
func (e errHandler) Handle(context.Context, slog.Record) error { return fmt.Errorf("fail") }
func (e errHandler) WithAttrs([]slog.Attr) slog.Handler        { return e }
func (e errHandler) WithGroup(string) slog.Handler             { return e }

func TestStreamHandler_NextHandlerError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(logger)
	streamer := NewLogStreamer(logger, hub)
	handler := streamer.StreamHandler(errHandler{})

	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	err := handler.Handle(context.Background(), rec)
	if err == nil || err.Error() != "fail" {
		t.Fatalf("expected 'fail' error, got %v", err)
	}
}

func TestStreamHandler_RecordWithAttrs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{id: "attr-test", send: make(chan []byte, 16), hub: hub}
	hub.register <- client
	time.Sleep(5 * time.Millisecond)

	streamer := NewLogStreamer(logger, hub)
	handler := streamer.StreamHandler(slog.NewTextHandler(io.Discard, nil))

	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "attr-msg", 0)
	rec.AddAttrs(slog.String("key1", "val1"), slog.Int("key2", 42))

	if err := handler.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-client.send:
		var entry LogEntry
		if err := json.Unmarshal(msg, &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Attrs["key1"] != "val1" {
			t.Errorf("expected key1=val1, got %v", entry.Attrs)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for streamed log with attrs")
	}
}

func TestStreamHandler_WithAttrsHandle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{id: "withattrs-test", send: make(chan []byte, 16), hub: hub}
	hub.register <- client
	time.Sleep(5 * time.Millisecond)

	streamer := NewLogStreamer(logger, hub)
	handler := streamer.StreamHandler(slog.NewTextHandler(io.Discard, nil))
	handler = handler.WithAttrs([]slog.Attr{slog.String("builtin", "yes")})

	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "withattrs-msg", 0)
	if err := handler.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-client.send:
		var entry LogEntry
		if err := json.Unmarshal(msg, &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Attrs["builtin"] != "yes" {
			t.Errorf("expected builtin=yes, got %v", entry.Attrs)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for streamed log with builtin attrs")
	}
}

// TestLogFilterMatches locks in OBS-LOG-1: empty filters match
// anything; non-empty filters require membership.
func TestLogFilterMatches(t *testing.T) {
	cases := []struct {
		name   string
		filter LogFilter
		entry  LogEntry
		want   bool
	}{
		{"empty-all", LogFilter{}, LogEntry{Level: "INFO", Source: "x"}, true},
		{"level-match", LogFilter{Levels: []string{"INFO"}}, LogEntry{Level: "INFO"}, true},
		{"level-miss", LogFilter{Levels: []string{"INFO"}}, LogEntry{Level: "WARN"}, false},
		{"source-match", LogFilter{Sources: []string{"auth"}}, LogEntry{Source: "auth"}, true},
		{"source-miss", LogFilter{Sources: []string{"auth"}}, LogEntry{Source: "vault"}, false},
		{"both-match", LogFilter{Levels: []string{"INFO"}, Sources: []string{"auth"}}, LogEntry{Level: "INFO", Source: "auth"}, true},
		{"both-half-miss", LogFilter{Levels: []string{"INFO"}, Sources: []string{"auth"}}, LogEntry{Level: "INFO", Source: "vault"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.filter.matches(&c.entry); got != c.want {
				t.Errorf("matches(%+v, %+v) = %v, want %v", c.filter, c.entry, got, c.want)
			}
		})
	}
}

// TestSplitCSV exercises the OBS-LOG-1 query-param parser.
func TestSplitCSV(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{",,,", nil},
		{"info", []string{"info"}},
		{"info,warn", []string{"info", "warn"}},
		{" info , warn ", []string{"info", "warn"}},
		{"info,,warn", []string{"info", "warn"}},
	}
	for _, c := range cases {
		if got := splitCSV(c.in); !equalStringSlice(got, c.want) {
			t.Errorf("splitCSV(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestHubBroadcastFiltersByLevel exercises the broadcast loop's
// per-client filter path: two clients, one with ?level=info
// filter, one without; only the unfiltered client sees the WARN
// entry.
func TestHubBroadcastFiltersByLevel(t *testing.T) {
	hub := NewHub(silentLogger())
	go hub.Run()
	defer hub.Stop()

	clientUnfiltered := hub.newTestClient(LogFilter{})
	clientInfoOnly := hub.newTestClient(LogFilter{Levels: []string{"INFO"}})
	hub.register <- clientUnfiltered
	hub.register <- clientInfoOnly
	// Allow register to drain.
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastLog(&LogEntry{Timestamp: time.Now(), Level: "WARN", Source: "auth", Message: "warn-1"})
	hub.BroadcastLog(&LogEntry{Timestamp: time.Now(), Level: "INFO", Source: "auth", Message: "info-1"})

	// Unfiltered client: receives both, in order.
	got1 := tryReceive(clientUnfiltered, time.Second)
	got2 := tryReceive(clientUnfiltered, time.Second)
	if !((got1 == "warn-1" && got2 == "info-1") || (got1 == "info-1" && got2 == "warn-1")) {
		t.Errorf("unfiltered client should see warn-1 and info-1, got %q %q", got1, got2)
	}
	// Info-only client: receives only info-1, not warn-1.
	got := tryReceive(clientInfoOnly, 500*time.Millisecond)
	if got != "info-1" {
		t.Errorf("info-only client should see info-1, got %q", got)
	}
	if extra := tryReceive(clientInfoOnly, 200*time.Millisecond); extra != "" {
		t.Errorf("info-only client should not see WARN, got %q", extra)
	}
}

// helpers --------------------------------------------------------------

func (h *Hub) newTestClient(filter LogFilter) *Client {
	h.mu.Lock()
	h.nextID++
	c := &Client{
		id:     fmt.Sprintf("test-client-%d", h.nextID),
		send:   make(chan []byte, 256),
		hub:    h,
		filter: filter,
	}
	h.mu.Unlock()
	return c
}

func assertMessage(t *testing.T, c *Client, want string) {
	t.Helper()
	select {
	case msg := <-c.send:
		var entry LogEntry
		if err := json.Unmarshal(msg, &entry); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if entry.Message != want {
			t.Errorf("expected message %q, got %q", want, entry.Message)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for %q", want)
	}
}

func tryReceive(c *Client, d time.Duration) string {
	select {
	case msg := <-c.send:
		var entry LogEntry
		if err := json.Unmarshal(msg, &entry); err != nil {
			return ""
		}
		return entry.Message
	case <-time.After(d):
		return ""
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestHubBroadcastNonBlockingDrops exercises OBS-LOG-2: when
// the broadcast channel is full, BroadcastLog returns without
// blocking and increments Dropped().
func TestHubBroadcastNonBlockingDrops(t *testing.T) {
	hub := NewHub(silentLogger())
	hub.broadcast = make(chan []byte, 2)
	// Don't start hub.Run; the broadcast channel is full and
	// nothing drains it.
	hub.BroadcastLog(&LogEntry{Timestamp: time.Now(), Level: "INFO", Message: "m1"})
	hub.BroadcastLog(&LogEntry{Timestamp: time.Now(), Level: "INFO", Message: "m2"})
	if got := hub.Dropped(); got != 0 {
		t.Errorf("expected 0 drops while channel had room, got %d", got)
	}
	hub.BroadcastLog(&LogEntry{Timestamp: time.Now(), Level: "INFO", Message: "m3"})
	if got := hub.Dropped(); got != 1 {
		t.Errorf("expected 1 drop after channel full, got %d", got)
	}
}

// fakeHubStore is the minimal HubStore stub for OBS-LOG-3 tests.
type fakeHubStore struct {
	mu       sync.Mutex
	nextID   int64
	failGet  bool
	failSet  bool
}

func (f *fakeHubStore) GetWSNextID(_ context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failGet {
		return 0, errors.New("fake get error")
	}
	return f.nextID, nil
}

func (f *fakeHubStore) SetWSNextID(_ context.Context, n int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failSet {
		return errors.New("fake set error")
	}
	f.nextID = n
	return nil
}

// TestHubSetStoreLoadsPersistedNextID locks in OBS-LOG-3:
// SetStore reads the persisted counter synchronously so the
// hub's nextID reflects the persisted value before the first
// HandleSSE call.
func TestHubSetStoreLoadsPersistedNextID(t *testing.T) {
	store := &fakeHubStore{nextID: 7}
	h := NewHub(silentLogger())
	if err := h.SetStore(context.Background(), store); err != nil {
		t.Fatalf("SetStore: %v", err)
	}
	if h.nextID != 7 {
		t.Errorf("expected nextID=7 after SetStore, got %d", h.nextID)
	}
}

// TestHubStopPersistsNextID exercises the Stop path: the final
// nextID is written back via SetWSNextID.
func TestHubStopPersistsNextID(t *testing.T) {
	store := &fakeHubStore{}
	h := NewHub(silentLogger())
	if err := h.SetStore(context.Background(), store); err != nil {
		t.Fatalf("SetStore: %v", err)
	}
	go h.Run()
	h.mu.Lock()
	h.nextID = 42
	h.mu.Unlock()
	h.Stop()
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.nextID != 42 {
		t.Errorf("expected persisted nextID=42, got %d", store.nextID)
	}
}

// TestHubSetStoreErrorDoesNotPanic ensures SetStore tolerates
// store failures: the hub falls back to process-local counters
// rather than crashing on startup.
func TestHubSetStoreErrorDoesNotPanic(t *testing.T) {
	h := NewHub(silentLogger())
	store := &fakeHubStore{failGet: true}
	if err := h.SetStore(context.Background(), store); err == nil {
		t.Fatal("expected error from SetStore when GetWSNextID fails")
	}
	// nextID stays at 1 (default).
	if h.nextID != 0 {
		t.Errorf("expected nextID=0 after failed SetStore, got %d", h.nextID)
	}
}
