package llm

import (
	"fmt"
	"os"
	"sync"
)

// Registry manages provider construction and lookup.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]func(ProviderConfig) Provider
	configs   map[string]ProviderConfig
	cache     map[string]Provider // cached provider instances
}

// Global is the default provider registry, pre-populated with all built-in providers.
var Global = newRegistry()

func newRegistry() *Registry {
	r := &Registry{
		providers: make(map[string]func(ProviderConfig) Provider),
		configs:   make(map[string]ProviderConfig),
		cache:     make(map[string]Provider),
	}
	r.Register("openai", func(cfg ProviderConfig) Provider { return NewOpenAI(cfg) })
	r.Register("anthropic", func(cfg ProviderConfig) Provider { return NewAnthropic(cfg) })
	r.Register("ollama", func(cfg ProviderConfig) Provider { return NewOllama(cfg) })
	r.Register("azure", func(cfg ProviderConfig) Provider { return NewAzure(cfg) })
	r.Register("nvidia", func(cfg ProviderConfig) Provider { return NewNvidia(cfg) })
	return r
}

// Register adds a provider factory under the given name.
func (r *Registry) Register(name string, factory func(ProviderConfig) Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = factory
	// Invalidate cache when factory changes
	delete(r.cache, name)
}

// Configure sets the config for a named provider and invalidates cache.
func (r *Registry) Configure(name string, cfg ProviderConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[name] = cfg
	// Invalidate cache when config changes
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

	// Double-checked locking: re-acquire write lock to avoid a race
	// where two concurrent calls both miss the cache and create
	// duplicate instances of the same provider.
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

// LoadFromEnv configures providers from environment variables.
// Supported env vars:
//
//	PROMPTSHEON_LLM_PROVIDER     — default provider name
//	PROMPTSHEON_OPENAI_API_KEY   — OpenAI API key
//	PROMPTSHEON_OPENAI_BASE_URL  — OpenAI base URL (optional)
//	PROMPTSHEON_ANTHROPIC_API_KEY — Anthropic API key
//	PROMPTSHEON_ANTHROPIC_BASE_URL — Anthropic base URL (optional)
//	PROMPTSHEON_OLLAMA_BASE_URL  — Ollama base URL (optional)
//	PROMPTSHEON_NVIDIA_API_KEY   — NVIDIA NIM API key
//	PROMPTSHEON_NVIDIA_BASE_URL  — NVIDIA NIM base URL (optional)
func LoadFromEnv() string {
	if v := os.Getenv("PROMPTSHEON_OPENAI_API_KEY"); v != "" {
		Global.Configure("openai", ProviderConfig{
			APIKey:  v,
			BaseURL: os.Getenv("PROMPTSHEON_OPENAI_BASE_URL"),
		})
	}
	if v := os.Getenv("PROMPTSHEON_ANTHROPIC_API_KEY"); v != "" {
		Global.Configure("anthropic", ProviderConfig{
			APIKey:  v,
			BaseURL: os.Getenv("PROMPTSHEON_ANTHROPIC_BASE_URL"),
		})
	}
	if v := os.Getenv("PROMPTSHEON_OLLAMA_BASE_URL"); v != "" {
		Global.Configure("ollama", ProviderConfig{
			BaseURL: v,
		})
	}
	if v := os.Getenv("PROMPTSHEON_AZURE_API_KEY"); v != "" {
		Global.Configure("azure", ProviderConfig{
			APIKey:  v,
			BaseURL: os.Getenv("PROMPTSHEON_AZURE_RESOURCE"),
			Extra: map[string]string{
				"deployment":  os.Getenv("PROMPTSHEON_AZURE_DEPLOYMENT"),
				"api_version": os.Getenv("PROMPTSHEON_AZURE_API_VERSION"),
			},
		})
	}
	if v := os.Getenv("PROMPTSHEON_NVIDIA_API_KEY"); v != "" {
		Global.Configure("nvidia", ProviderConfig{
			APIKey:  v,
			BaseURL: os.Getenv("PROMPTSHEON_NVIDIA_BASE_URL"),
		})
	}

	return os.Getenv("PROMPTSHEON_LLM_PROVIDER")
}
