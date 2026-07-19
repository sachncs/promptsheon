package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/sachncs/promptsheon/internal/webhook"
)

func (s *Server) handleListWebhooks(w http.ResponseWriter, _ *http.Request) error {
	if s.webhooks == nil {
		writeJSON(w, http.StatusOK, []any{})
		return nil
	}
	eps := s.webhooks.ListEndpoints()
	writeJSON(w, http.StatusOK, eps)
	return nil
}

func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) error {
	if s.webhooks == nil {
		return &HTTPError{Status: http.StatusServiceUnavailable, Message: "webhook dispatcher not configured"}
	}
	var req struct {
		URL           string              `json:"url"`
		Events        []webhook.EventType `json:"events"`
		Secret        string              `json:"secret,omitempty"`
		AllowPrivate  bool                `json:"allow_private,omitempty"`
		AllowInsecure bool                `json:"allow_insecure,omitempty"`
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
	if err := ValidateWebhookURL(req.URL, req.AllowPrivate); err != nil {
		return &HTTPError{Status: http.StatusBadRequest, Message: fmt.Sprintf("invalid url: %v", err)}
	}
	ep := &webhook.Endpoint{
		ID:            generateID(),
		URL:           req.URL,
		Secret:        req.Secret,
		AllowPrivate:  req.AllowPrivate,
		AllowInsecure: req.AllowInsecure,
		Events:        req.Events,
		Active:        true,
	}
	s.webhooks.Register(ep)
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
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
	return nil
}

// ValidateWebhookURL performs the SSRF protection for inbound webhook
// registrations. The check rejects:
//
//   - non-http(s) schemes
//   - URLs that resolve to loopback / private / link-local IPs
//     (including cloud metadata endpoints like 169.254.169.254)
//   - hosts that are themselves loopback names
//
// The previous global PROMPTSHEON_WEBHOOK_ALLOW_PRIVATE env var was
// removed: SSRF protection is now per-endpoint. Callers that
// legitimately need to deliver to loopback or RFC1918 hosts set
// AllowPrivate=true on the request; the value is recorded in the
// audit log.
func ValidateWebhookURL(rawURL string, allowPrivate bool) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q (must be http or https)", u.Scheme)
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
		if !allowPrivate {
			return fmt.Errorf("loopback / metadata hostnames are not allowed")
		}
	}

	// Resolve every IP that the host maps to. Reject the URL if any of
	// them is in a blocked range. This is the standard DNS-rebinding
	// mitigation: validate at registration time AND at delivery time.
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolve host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("host did not resolve")
	}
	if allowPrivate {
		return nil
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
func ResolveAndValidateWebhook(ctx context.Context, rawURL string, allowPrivate bool) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
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
	if allowPrivate {
		return nil
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return fmt.Errorf("host resolves to disallowed address %s", ip)
		}
	}
	return nil
}
