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
}

// Global is the default registry.
var Global = newRegistry()

func newRegistry() *Registry {
	r := &Registry{
		providers: make(map[string]func(ProviderConfig) Provider),
		configs:   make(map[string]ProviderConfig),
	}
	r.Register("openai", func(cfg ProviderConfig) Provider { return NewOpenAI(cfg) })
	r.Register("anthropic", func(cfg ProviderConfig) Provider { return NewAnthropic(cfg) })
	r.Register("ollama", func(cfg ProviderConfig) Provider { return NewOllama(cfg) })
	r.Register("azure", func(cfg ProviderConfig) Provider { return NewAzure(cfg) })
	return r
}

// Register adds a provider factory under the given name.
func (r *Registry) Register(name string, factory func(ProviderConfig) Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = factory
}

// Configure sets the config for a named provider.
func (r *Registry) Configure(name string, cfg ProviderConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[name] = cfg
}

// Get returns a provider by name, using the configured config.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	factory, ok := r.providers[name]
	cfg, _ := r.configs[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return factory(cfg), nil
}

// GetWithConfig returns a provider with an explicit config override.
func (r *Registry) GetWithConfig(name string, cfg ProviderConfig) (Provider, error) {
	r.mu.RLock()
	factory, ok := r.providers[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return factory(cfg), nil
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
				"deployment": os.Getenv("PROMPTSHEON_AZURE_DEPLOYMENT"),
				"api_version": os.Getenv("PROMPTSHEON_AZURE_API_VERSION"),
			},
		})
	}

	return os.Getenv("PROMPTSHEON_LLM_PROVIDER")
}
