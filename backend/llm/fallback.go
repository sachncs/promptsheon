package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Fallback wraps a primary Provider with automatic failover to fallback providers.
// If the primary fails, it tries each fallback in order until one succeeds.
type Fallback struct {
	primary   Provider
	fallbacks []Provider
	logger    *slog.Logger
}

// NewFallback creates a new fallback provider chain.
// primary is tried first; if it fails, fallbacks are tried in order.
func NewFallback(primary Provider, fallbacks []Provider, logger *slog.Logger) *Fallback {
	return &Fallback{
		primary:   primary,
		fallbacks: fallbacks,
		logger:    logger,
	}
}

// Complete implements Provider. Tries primary first, then fallbacks.
func (f *Fallback) Complete(ctx context.Context, req *Request) (*Response, error) {
	// Try primary
	resp, err := f.primary.Complete(ctx, req)
	if err == nil {
		return resp, nil
	}

	if f.logger != nil {
		f.logger.Warn("primary provider failed, trying fallbacks",
			"primary", f.primary.Name(),
			"error", err,
		)
	}

	// Try fallbacks in order
	for _, fb := range f.fallbacks {
		if fb.Name() == f.primary.Name() {
			continue // skip if fallback is same as primary
		}

		resp, err = fb.Complete(ctx, req)
		if err == nil {
			if f.logger != nil {
				f.logger.Info("fallback provider succeeded",
					"provider", fb.Name(),
				)
			}
			return resp, nil
		}

		if f.logger != nil {
			f.logger.Warn("fallback provider also failed",
				"provider", fb.Name(),
				"error", err,
			)
		}
	}

	return nil, fmt.Errorf("all providers failed, last error: %w", err)
}

// Name returns the primary provider name with fallback indicator.
func (f *Fallback) Name() string {
	names := make([]string, 0, 1+len(f.fallbacks))
	names = append(names, f.primary.Name())
	for _, fb := range f.fallbacks {
		names = append(names, fb.Name())
	}
	return fmt.Sprintf("fallback(%s)", strings.Join(names, ","))
}

// Fallback satisfies the Provider interface.
var _ Provider = (*Fallback)(nil)

// ParseFallbackProviders parses a comma-separated list of provider names
// and returns them as a slice. Empty strings are filtered out.
func ParseFallbackProviders(names string) []string {
	if names == "" {
		return nil
	}
	parts := strings.Split(names, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
