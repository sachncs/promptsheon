// Package webhook provides event delivery to external systems via HTTP.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// EventType identifies the kind of event that triggered a webhook.
type EventType string

const (
	// EventEvalCompleted is emitted when an evaluation completes.
	EventEvalCompleted EventType = "eval.completed"
	// EventReviewApproved is emitted when a review is approved.
	EventReviewApproved EventType = "review.approved"
	// EventReviewRejected is emitted when a review is rejected.
	EventReviewRejected EventType = "review.rejected"
	// EventWorkflowCompleted is emitted when a workflow completes.
	EventWorkflowCompleted EventType = "workflow.completed"
	// EventWorkflowFailed is emitted when a workflow fails.
	EventWorkflowFailed EventType = "workflow.failed"
	// EventPromptCreated is emitted when a prompt is created.
	EventPromptCreated EventType = "prompt.created"
	// EventPromptUpdated is emitted when a prompt is updated.
	EventPromptUpdated EventType = "prompt.updated"
	// EventPromptDeployed is emitted when a prompt is deployed.
	EventPromptDeployed EventType = "prompt.deployed"
	// EventPromptArchived is emitted when a prompt is archived.
	EventPromptArchived EventType = "prompt.archived"
)

// Event is the payload sent to webhook endpoints.
type Event struct {
	ID        string         `json:"id"`
	Type      EventType      `json:"type"`
	Resource  string         `json:"resource"`
	Data      map[string]any `json:"data"`
	Timestamp time.Time      `json:"timestamp"`
}

// Endpoint represents a configured webhook destination.
type Endpoint struct {
	ID            string      `json:"id"`
	URL           string      `json:"url"`
	Secret        string      `json:"secret,omitempty"` // for HMAC signing
	AllowInsecure bool        `json:"allow_insecure"`
	AllowPrivate  bool        `json:"allow_private"`
	Events        []EventType `json:"events"`
	Active        bool        `json:"active"`
	CreatedAt     time.Time   `json:"created_at"`
}

// Delivery records the outcome of a webhook delivery attempt.
type Delivery struct {
	ID         string    `json:"id"`
	EndpointID string    `json:"endpoint_id"`
	EventID    string    `json:"event_id"`
	StatusCode int       `json:"status_code"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	Attempts   int       `json:"attempts"`
	CreatedAt  time.Time `json:"created_at"`
}

// EndpointStore is the persistence contract for webhook endpoints. The
// dispatcher is decoupled from a particular store so it can be tested
// with an in-memory implementation.
type EndpointStore interface {
	SaveWebhookEndpoint(ctx context.Context, ep *Endpoint) error
	DeleteWebhookEndpoint(ctx context.Context, id string) error
	ListWebhookEndpoints(ctx context.Context) ([]*Endpoint, error)
}

// Dispatcher delivers events to registered webhook endpoints.
type Dispatcher struct {
	mu             sync.RWMutex
	endpoints      map[string]*Endpoint
	store          EndpointStore
	client         *http.Client
	logger         *slog.Logger
	deliveries     []*Delivery
	deliveriesHead int
	deliveriesLen  int
	maxRetries     int
	maxDeliveries  int
}

// NewDispatcher creates a webhook dispatcher.
func NewDispatcher(logger *slog.Logger) *Dispatcher {
	d := &Dispatcher{
		endpoints:     make(map[string]*Endpoint),
		client:        &http.Client{Timeout: 10 * time.Second},
		logger:        logger,
		maxRetries:    3,
		maxDeliveries: 1000,
	}
	return d
}

// WithEndpointStore attaches a persistence backend. When set, Register
// and Remove are also written to the store, and LoadFromStore hydrates
// the in-memory map.
func (d *Dispatcher) WithEndpointStore(s EndpointStore) *Dispatcher {
	d.store = s
	return d
}

// LoadFromStore populates the in-memory endpoint map from the store.
// Intended to be called once at server startup.
func (d *Dispatcher) LoadFromStore(ctx context.Context) error {
	if d.store == nil {
		return nil
	}
	eps, err := d.store.ListWebhookEndpoints(ctx)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, ep := range eps {
		d.endpoints[ep.ID] = ep
	}
	return nil
}

// WithMaxRetries sets the maximum number of delivery retries.
func (d *Dispatcher) WithMaxRetries(n int) *Dispatcher {
	d.maxRetries = n
	return d
}

// WithHTTPClient replaces the default HTTP client. Useful for tests
// and for operators who need to customise transport (TLS, proxy,
// connection pool).
func (d *Dispatcher) WithHTTPClient(c *http.Client) *Dispatcher {
	if c != nil {
		d.client = c
	}
	return d
}

// Register adds a webhook endpoint. When a store is configured the
// endpoint is also written to it so it survives a restart.
func (d *Dispatcher) Register(ep *Endpoint) {
	d.mu.Lock()
	d.endpoints[ep.ID] = ep
	d.mu.Unlock()
	if d.store != nil {
		if err := d.store.SaveWebhookEndpoint(context.Background(), ep); err != nil && d.logger != nil {
			d.logger.Error("webhook: persist endpoint failed", "err", err, "id", ep.ID)
		}
	}
}

// Remove deletes a webhook endpoint. When a store is configured the
// removal is also persisted.
func (d *Dispatcher) Remove(id string) {
	d.mu.Lock()
	delete(d.endpoints, id)
	d.mu.Unlock()
	if d.store != nil {
		if err := d.store.DeleteWebhookEndpoint(context.Background(), id); err != nil && d.logger != nil {
			d.logger.Error("webhook: persist delete failed", "err", err, "id", id)
		}
	}
}

// ListEndpoints returns all registered endpoints.
func (d *Dispatcher) ListEndpoints() []*Endpoint {
	d.mu.RLock()
	defer d.mu.RUnlock()
	eps := make([]*Endpoint, 0, len(d.endpoints))
	for _, ep := range d.endpoints {
		eps = append(eps, ep)
	}
	return eps
}

// ListDeliveries returns the delivery log (oldest first).
func (d *Dispatcher) ListDeliveries() []*Delivery {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]*Delivery, 0, d.deliveriesLen)
	for i := 0; i < d.deliveriesLen; i++ {
		if d.deliveriesLen < cap(d.deliveries) {
			out = append(out, d.deliveries[i])
		} else {
			idx := (d.deliveriesHead + i) % cap(d.deliveries)
			out = append(out, d.deliveries[idx])
		}
	}
	return out
}

// Emit sends an event to all matching endpoints asynchronously.
func (d *Dispatcher) Emit(evt *Event) {
	d.EmitContext(context.Background(), evt)
}

// EmitContext is the ctx-aware variant of Emit. The returned context
// is the parent of all per-endpoint delivery goroutines; cancelling it
// aborts in-flight deliveries.
func (d *Dispatcher) EmitContext(ctx context.Context, evt *Event) {
	d.mu.RLock()
	var targets []*Endpoint
	for _, ep := range d.endpoints {
		if !ep.Active {
			continue
		}
		for _, e := range ep.Events {
			if e == evt.Type {
				targets = append(targets, ep)
				break
			}
		}
	}
	d.mu.RUnlock()

	for _, ep := range targets {
		go d.deliver(ctx, ep, evt)
	}
}

// ValidateURL is the dispatcher-side SSRF check. The previous
// implementation honoured PROMPTSHEON_WEBHOOK_ALLOW_PRIVATE as a
// global env-var override; that toggle was an SSRF enabler (any
// caller could trigger webhook deliveries to private addresses once
// the env was set). The new model is per-endpoint: callers that
// legitimately need to deliver to loopback or RFC1918 hosts set
// AllowPrivate=true on the endpoint at registration, which is
// recorded in the audit log.
//
// handlers_webhooks.go runs the same check at registration time.
// The dispatcher re-runs ValidateURLFor every delivery to defeat
// DNS rebinding between registration and delivery.
func ValidateURL(rawURL string) error { return ValidateURLFor(rawURL, false) }

// ValidateURLFor mirrors ValidateURL but lets the caller pass the
// per-endpoint AllowPrivate flag.
func ValidateURLFor(rawURL string, allowPrivate bool) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	if allowPrivate {
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("host %s resolves to disallowed address %s", host, ip)
		}
	}
	return nil
}

func (d *Dispatcher) deliver(ctx context.Context, ep *Endpoint, evt *Event) {
	// Re-validate the URL at delivery time to defeat DNS rebinding.
	if err := ValidateURLFor(ep.URL, ep.AllowPrivate); err != nil {
		d.recordDelivery(&Delivery{
			ID:         generateID(),
			EndpointID: ep.ID,
			EventID:    evt.ID,
			Success:    false,
			Error:      "url validation failed: " + err.Error(),
			Attempts:   0,
			CreatedAt:  time.Now(),
		})
		return
	}

	body, err := json.Marshal(evt)
	if err != nil {
		d.logger.Error("webhook marshal failed", "endpoint", ep.ID, "err", err)
		return
	}

	var signature string
	if ep.Secret != "" {
		mac := hmac.New(sha256.New, []byte(ep.Secret))
		mac.Write(body)
		signature = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	var lastErr error
	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			lastErr = err
			break
		}
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, "POST", ep.URL, bytes.NewReader(body))
		if err != nil {
			cancel()
			lastErr = err
			break
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Promptsheon-Event", string(evt.Type))
		req.Header.Set("X-Promptsheon-Delivery", evt.ID)
		if signature != "" {
			req.Header.Set("X-Promptsheon-Signature", signature)
		}

		resp, err := d.client.Do(req)
		cancel()
		if err != nil {
			lastErr = err
			if !sleepBackoff(ctx, attempt) {
				break
			}
			continue
		}
		if _, err := io.Copy(io.Discard, resp.Body); err != nil && d.logger != nil {
			d.logger.Warn("webhook: drain response body failed", "endpoint", ep.ID, "err", err)
		}
		if err := resp.Body.Close(); err != nil && d.logger != nil {
			d.logger.Warn("webhook: close response body failed", "endpoint", ep.ID, "err", err)
		}

		delivery := Delivery{
			ID:         generateID(),
			EndpointID: ep.ID,
			EventID:    evt.ID,
			StatusCode: resp.StatusCode,
			Success:    resp.StatusCode >= 200 && resp.StatusCode < 300,
			Attempts:   attempt + 1,
			CreatedAt:  time.Now(),
		}
		if !delivery.Success {
			delivery.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
			lastErr = errors.New(delivery.Error)
			if !sleepBackoff(ctx, attempt) {
				break
			}
			continue
		}

		d.recordDelivery(&delivery)
		return
	}

	d.recordDelivery(&Delivery{
		ID:         generateID(),
		EndpointID: ep.ID,
		EventID:    evt.ID,
		Success:    false,
		Error:      fmt.Sprintf("after %d retries: %v", d.maxRetries, lastErr),
		Attempts:   d.maxRetries + 1,
		CreatedAt:  time.Now(),
	})
}

// sleepBackoff implements truncated exponential backoff with jitter.
// Returns false if the context was cancelled during the sleep.
func sleepBackoff(ctx context.Context, attempt int) bool {
	if attempt < 0 {
		attempt = 0
	}
	base := 250 * time.Millisecond
	maxd := 30 * time.Second
	d := base << attempt
	if d > maxd || d < 0 {
		d = maxd
	}
	// #nosec G404 -- jitter uses math/rand/v2, not crypto/rand;
	// cryptographic randomness is not needed for backoff timing.
	jitter := time.Duration(rand.Int64N(int64(d) / 2))
	d += jitter
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// recordDelivery appends to a fixed-capacity ring buffer. The previous
// implementation used a slice with head-shift truncation, which is
// O(n) on every overflow; the ring buffer is O(1).
func (d *Dispatcher) recordDelivery(delivery *Delivery) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.deliveries == nil {
		d.deliveries = make([]*Delivery, d.maxDeliveries)
		d.deliveriesHead = 0
		d.deliveriesLen = 0
	}
	if cap(d.deliveries) == 0 {
		d.deliveries = make([]*Delivery, d.maxDeliveries)
	}
	if d.deliveriesLen < cap(d.deliveries) {
		idx := (d.deliveriesHead + d.deliveriesLen) % cap(d.deliveries)
		d.deliveries[idx] = delivery
		d.deliveriesLen++
	} else {
		d.deliveries[d.deliveriesHead] = delivery
		d.deliveriesHead = (d.deliveriesHead + 1) % cap(d.deliveries)
	}
}

func generateID() string {
	return fmt.Sprintf("wh-%d", time.Now().UnixNano())
}
