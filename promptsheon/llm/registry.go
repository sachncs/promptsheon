package llm

import (
	"log/slog"
	"net/url"
	"os"
	"sync"

	"github.com/sachncs/promptsheon/errf"
)

// Registry manages provider construction and lookup.
//
// A Registry is an explicit value owned by the caller; it is not a
// package-level singleton. Wiring is performed at process startup
// (cmd/promptsheond or cmd/promptsheon) and the resulting Registry is
// passed to consumers via dependency injection. See ADR-0012 for the
// rationale.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]func(ProviderConfig) Provider
	configs   map[string]ProviderConfig
	cache     map[string]Provider
}

// NewRegistry constructs a Registry pre-populated with the built-in
// providers (openai, anthropic). A fresh Registry is safe for
// concurrent use.
//
// Tests and embedders can construct their own Registry and Register
// only the providers they need.
func NewRegistry() *Registry {
	r := &Registry{
		providers: make(map[string]func(ProviderConfig) Provider),
		configs:   make(map[string]ProviderConfig),
		cache:     make(map[string]Provider),
	}
	r.Register("openai", func(cfg ProviderConfig) Provider { return NewOpenAI(cfg) })
	r.Register("anthropic", func(cfg ProviderConfig) Provider { return NewAnthropic(cfg) })
	return r
}

// Register adds a provider factory under the given name.
func (r *Registry) Register(name string, factory func(ProviderConfig) Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = factory
	delete(r.cache, name)
}

// Configure sets the config for a named provider and invalidates the cache.
func (r *Registry) Configure(name string, cfg ProviderConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[name] = cfg
	delete(r.cache, name)
}

// Get returns a cached provider by name, or creates a new one.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	if p, ok := r.cache[name]; ok {
		r.mu.RUnlock()
		return p, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if p, ok := r.cache[name]; ok {
		return p, nil
	}

	factory, ok := r.providers[name]
	if !ok {
		return nil, errf.Errorf("unknown provider: %s", name)
	}
	cfg, cfgOK := r.configs[name]
	if !cfgOK {
		return nil, errf.Errorf("provider %s not configured", name)
	}

	p := factory(cfg)
	r.cache[name] = p
	return p, nil
}

// Providers returns the names of all registered providers.
func (r *Registry) Providers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// LoadFromEnv configures providers from environment variables on this
// Registry instance (not a package-level singleton). Supported env vars:
//
//	PROMPTSHEON_LLM_PROVIDER      — default provider name
//	PROMPTSHEON_OPENAI_API_KEY    — OpenAI API key
//	PROMPTSHEON_OPENAI_BASE_URL   — OpenAI base URL (optional)
//	PROMPTSHEON_ANTHROPIC_API_KEY — Anthropic API key
//	PROMPTSHEON_ANTHROPIC_BASE_URL — Anthropic base URL (optional)
//
// Returns the value of PROMPTSHEON_LLM_PROVIDER (empty string when
// unset) for callers that want to default to a specific provider.
// LoadFromEnv configures both providers from environment variables.
// Returns the preferred default provider name (PROMPTSHEON_LLM_PROVIDER)
// for callers that want to know which provider to invoke.
// bindAddr / isLoopback are used to validate PROMPTSHEON_*_BASE_URL:
// http base URLs are only allowed on loopback binds.
func (r *Registry) LoadFromEnv(bindAddr string, isLoopback func(string) bool) string {
	if v := os.Getenv("PROMPTSHEON_OPENAI_API_KEY"); v != "" {
		if err := loadFromEnvBaseURL("openai", os.Getenv("PROMPTSHEON_OPENAI_BASE_URL"), bindAddr, isLoopback); err != nil {
			slog.Error("openai base url", "err", err)
		} else {
			r.Configure("openai", ProviderConfig{
				APIKey:  v,
				BaseURL: os.Getenv("PROMPTSHEON_OPENAI_BASE_URL"),
			})
		}
	}
	if v := os.Getenv("PROMPTSHEON_ANTHROPIC_API_KEY"); v != "" {
		if err := loadFromEnvBaseURL("anthropic", os.Getenv("PROMPTSHEON_ANTHROPIC_BASE_URL"), bindAddr, isLoopback); err != nil {
			slog.Error("anthropic base url", "err", err)
		} else {
			r.Configure("anthropic", ProviderConfig{
				APIKey:  v,
				BaseURL: os.Getenv("PROMPTSHEON_ANTHROPIC_BASE_URL"),
			})
		}
	}

	return os.Getenv("PROMPTSHEON_LLM_PROVIDER")
}

// loadFromEnvBaseURL validates an individual provider's BaseURL.
// Inlined from the old ValidateBaseURLs method (c2.24). The
// provider constructors (NewOpenAI / NewAnthropic) call this at
// registration time so a misconfigured base URL fails fast.
func loadFromEnvBaseURL(name, baseURL, bindAddr string, isLoopback func(string) bool) error {
	if baseURL == "" {
		return nil
	}
	if isLoopback == nil {
		isLoopback = func(string) bool { return true }
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return errf.Errorf("provider %q base url: %w", name, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errf.Errorf("provider %q base url scheme %q is not http or https", name, u.Scheme)
	}
	if u.Scheme == "http" && !isLoopback(bindAddr) {
		return errf.Errorf("provider %q base url %q uses http but daemon binds %q (non-loopback); http base URLs are only allowed on loopback binds", name, baseURL, bindAddr)
	}
	return nil
}
