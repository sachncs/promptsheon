// Package webhook provides event delivery to external systems via HTTP.
package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// EventType identifies the kind of event that triggered a webhook.
type EventType string

const (
	EventEvalCompleted     EventType = "eval.completed"
	EventReviewApproved    EventType = "review.approved"
	EventReviewRejected    EventType = "review.rejected"
	EventWorkflowCompleted EventType = "workflow.completed"
	EventWorkflowFailed    EventType = "workflow.failed"
	EventPromptCreated     EventType = "prompt.created"
	EventPromptUpdated     EventType = "prompt.updated"
	EventPromptDeployed    EventType = "prompt.deployed"
	EventPromptArchived    EventType = "prompt.archived"
)

// Event is the payload sent to webhook endpoints.
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Resource  string                 `json:"resource"`
	Data      map[string]any         `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// Endpoint represents a configured webhook destination.
type Endpoint struct {
	ID        string     `json:"id"`
	URL       string     `json:"url"`
	Secret    string     `json:"secret,omitempty"` // for HMAC signing
	Events    []EventType `json:"events"`
	Active    bool       `json:"active"`
	CreatedAt time.Time  `json:"created_at"`
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

// Dispatcher delivers events to registered webhook endpoints.
type Dispatcher struct {
	mu       sync.RWMutex
	endpoints map[string]*Endpoint
	client   *http.Client
	logger   *slog.Logger
	deliveries []Delivery
	maxRetries int
}

// NewDispatcher creates a webhook dispatcher.
func NewDispatcher(logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		endpoints:  make(map[string]*Endpoint),
		client:     &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
		maxRetries: 3,
	}
}

// WithMaxRetries sets the maximum number of delivery retries.
func (d *Dispatcher) WithMaxRetries(n int) *Dispatcher {
	d.maxRetries = n
	return d
}

// Register adds a webhook endpoint.
func (d *Dispatcher) Register(ep *Endpoint) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.endpoints[ep.ID] = ep
}

// Remove deletes a webhook endpoint.
func (d *Dispatcher) Remove(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.endpoints, id)
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

// ListDeliveries returns the delivery log.
func (d *Dispatcher) ListDeliveries() []Delivery {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Delivery, len(d.deliveries))
	copy(out, d.deliveries)
	return out
}

// Emit sends an event to all matching endpoints asynchronously.
func (d *Dispatcher) Emit(evt Event) {
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
		go d.deliver(ep, evt)
	}
}

func (d *Dispatcher) deliver(ep *Endpoint, evt Event) {
	body, err := json.Marshal(evt)
	if err != nil {
		d.logger.Error("webhook marshal failed", "endpoint", ep.ID, "err", err)
		return
	}

	var lastErr error
	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		req, err := http.NewRequest("POST", ep.URL, bytes.NewReader(body))
		if err != nil {
			lastErr = err
			break
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Promptsheon-Event", string(evt.Type))
		req.Header.Set("X-Promptsheon-Delivery", evt.ID)

		// HMAC signature
		if ep.Secret != "" {
			mac := hmac.New(sha256.New, []byte(ep.Secret))
			mac.Write(body)
			sig := hex.EncodeToString(mac.Sum(nil))
			req.Header.Set("X-Promptsheon-Signature", "sha256="+sig)
		}

		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()

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
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		d.mu.Lock()
		d.deliveries = append(d.deliveries, delivery)
		d.mu.Unlock()
		return
	}

	// All retries exhausted
	d.mu.Lock()
	d.deliveries = append(d.deliveries, Delivery{
		ID:         generateID(),
		EndpointID: ep.ID,
		EventID:    evt.ID,
		Success:    false,
		Error:      fmt.Sprintf("after %d retries: %v", d.maxRetries, lastErr),
		Attempts:   d.maxRetries + 1,
		CreatedAt:  time.Now(),
	})
	d.mu.Unlock()
}

func generateID() string {
	return fmt.Sprintf("wh-%d", time.Now().UnixNano())
}
