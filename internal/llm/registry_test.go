package llm

import (
	"sync"
	"testing"
)

func TestRegistryGetWithConfig(t *testing.T) {
	r := NewRegistry()
	r.Configure("openai", ProviderConfig{APIKey: "sk-test"})
	p, err := r.Get("openai")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestRegistryGetNoConfig(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("openai")
	if err == nil {
		t.Fatal("expected error for unconfigured provider")
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestRegistryConfigureInvalidatesCache(t *testing.T) {
	r := NewRegistry()
	r.Configure("openai", ProviderConfig{APIKey: "sk-old"})
	p1, err := r.Get("openai")
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	r.Configure("openai", ProviderConfig{APIKey: "sk-new"})
	p2, err := r.Get("openai")
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if p1 == p2 {
		t.Fatal("expected different instance after reconfiguration")
	}
}

func TestRegistryConcurrentGet(t *testing.T) {
	r := NewRegistry()
	r.Configure("openai", ProviderConfig{APIKey: "sk-concurrent"})

	var (
		wg      sync.WaitGroup
		errs    = make(chan error, 20)
		zeroErr = make(chan struct{}, 20)
	)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := r.Get("openai")
			if err != nil {
				errs <- err
				return
			}
			if p == nil {
				zeroErr <- struct{}{}
			}
		}()
	}
	wg.Wait()
	close(errs)
	close(zeroErr)

	for err := range errs {
		t.Errorf("concurrent Get: %v", err)
	}
	if len(zeroErr) > 0 {
		t.Errorf("concurrent Get returned nil %d time(s)", len(zeroErr))
	}
}

func TestRegistryRegisterInvalidatesCache(t *testing.T) {
	r := NewRegistry()
	r.Configure("custom", ProviderConfig{APIKey: "sk-custom"})
	// Register a factory that would be needed for "custom"
	r.Register("custom", func(cfg ProviderConfig) Provider {
		return NewOpenAI(cfg)
	})
	p, err := r.Get("custom")
	if err != nil {
		t.Fatalf("Get after Register: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}
