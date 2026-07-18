package llm

import (
	"fmt"
	"os"
	"sync"
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
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	cfg, cfgOK := r.configs[name]
	if !cfgOK {
		return nil, fmt.Errorf("provider %s not configured", name)
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
func (r *Registry) LoadFromEnv() string {
	if v := os.Getenv("PROMPTSHEON_OPENAI_API_KEY"); v != "" {
		r.Configure("openai", ProviderConfig{
			APIKey:  v,
			BaseURL: os.Getenv("PROMPTSHEON_OPENAI_BASE_URL"),
		})
	}
	if v := os.Getenv("PROMPTSHEON_ANTHROPIC_API_KEY"); v != "" {
		r.Configure("anthropic", ProviderConfig{
			APIKey:  v,
			BaseURL: os.Getenv("PROMPTSHEON_ANTHROPIC_BASE_URL"),
		})
	}

	return os.Getenv("PROMPTSHEON_LLM_PROVIDER")
}
