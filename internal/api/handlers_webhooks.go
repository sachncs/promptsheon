package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/webhook"
)

func (s *Server) handleListWebhooks(w http.ResponseWriter, _ *http.Request) error {
	if s.webhooks == nil {
		writeJSON(w, http.StatusOK, []any{})
		return nil
	}
	eps := s.webhooks.ListEndpoints()
	// SEC-7b: redact the secret on the read path. The endpoint in
	// memory may carry a plaintext Secret (set at registration
	// before the encryption pass), but the API must never echo
	// it back. Project a stable response shape that only exposes
	// a SecretSet boolean.
	type publicEndpoint struct {
		ID        string              `json:"id"`
		URL       string              `json:"url"`
		Events    []webhook.EventType `json:"events"`
		Active    bool                `json:"active"`
		SecretSet bool                `json:"secret_set"`
	}
	out := make([]publicEndpoint, 0, len(eps))
	for _, ep := range eps {
		out = append(out, publicEndpoint{
			ID:        ep.ID,
			URL:       ep.URL,
			Events:    ep.Events,
			Active:    ep.Active,
			SecretSet: ep.Secret != "",
		})
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) error {
	if s.webhooks == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "webhook dispatcher not configured"}
	}
	var req struct {
		URL    string              `json:"url"`
		Events []webhook.EventType `json:"events"`
		Secret string              `json:"secret,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return &HTTPError{Status: http.StatusBadRequest, Message: "invalid request body"}
	}
	if req.URL == "" {
		return &HTTPError{Status: http.StatusBadRequest, Message: "url is required"}
	}
	if len(req.Events) == 0 {
		return &HTTPError{Status: http.StatusBadRequest, Message: "at least one event is required"}
	}
	if err := ValidateWebhookURL(req.URL); err != nil {
		return &HTTPError{Status: http.StatusBadRequest, Message: fmt.Sprintf("invalid url: %v", err)}
	}
	var secretCiphertext []byte
	if req.Secret != "" {
		if s.vault == nil {
			return &HTTPError{Status: http.StatusServiceUnavailable, Message: "vault not configured; cannot store webhook secret"}
		}
		ct, err := s.vault.EncryptBytes([]byte(req.Secret))
		if err != nil {
			return fmt.Errorf("encrypt webhook secret: %w", err)
		}
		secretCiphertext = ct
	}
	ep := &webhook.Endpoint{
		ID:     generateID(),
		URL:    req.URL,
		Secret: req.Secret,
		Events: req.Events,
		Active: true,
	}
	eventStrs := make([]string, 0, len(ep.Events))
	for _, e := range ep.Events {
		eventStrs = append(eventStrs, string(e))
	}
	if err := s.db.SaveWebhookEndpoint(r.Context(), &models.WebhookEndpointRecord{
		ID:               ep.ID,
		URL:              ep.URL,
		Secret:           "", // plaintext never persisted
		SecretCiphertext: secretCiphertext,
		Events:           eventStrs,
		Active:           ep.Active,
		CreatedAt:        time.Now(),
	}); err != nil {
		return fmt.Errorf("save webhook endpoint: %w", err)
	}
	s.webhooks.Register(ep)
	s.audit(r.Context(), "webhook_create", "webhook", map[string]any{
		"id":     ep.ID,
		"url":    ep.URL,
		"events": ep.Events,
	})
	writeJSON(w, http.StatusCreated, ep)
	return nil
}

func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) error {
	if s.webhooks == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "webhook dispatcher not configured"}
	}
	id := r.PathValue("id")
	if id == "" {
		return &HTTPError{Status: http.StatusBadRequest, Message: "id is required"}
	}
	s.webhooks.Remove(id)
	s.audit(r.Context(), "webhook_delete", "webhook", map[string]any{
		"id": id,
	})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ValidateWebhookURL performs the SSRF protection for inbound webhook
// registrations. Only https is accepted and the host must resolve
// to a non-private, non-loopback, non-link-local, non-multicast,
// non-unspecified address. The previous per-endpoint allow_private
// and allow_insecure flags were removed (SEC-4, SEC-11); no
// caller can dial a private or http:// destination through this
// surface.
func ValidateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q (only https is accepted)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	// Reject obvious loopback / metadata host names even if we cannot
	// resolve them, so an operator cannot register `localhost` and have
	// it pass on a system where the resolution is slow / cached.
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") ||
		lower == "metadata.google.internal" || strings.HasSuffix(lower, ".internal") {
		return fmt.Errorf("loopback / metadata hostnames are not allowed")
	}

	// Resolve every IP that the host maps to. Reject the URL if any of
	// them is in a blocked range. This is the standard DNS-rebinding
	// mitigation: validate at registration time AND at delivery time.
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolve host: %v", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("host did not resolve")
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("host resolves to disallowed address %s", ip)
		}
	}
	return nil
}

// ResolveAndValidateWebhook is the same as ValidateWebhookURL but is
// intended to be called at delivery time as a DNS-rebinding defence: the
// IP set can change between registration and invocation.
func ResolveAndValidateWebhook(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q (only https is accepted)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	resolver := &net.Resolver{}
	ips, err := resolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return fmt.Errorf("host resolves to disallowed address %s", ip)
		}
	}
	return nil
}
