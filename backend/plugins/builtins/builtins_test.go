package builtins

import (
	"context"
	"testing"
)

func TestPIIDetectorLifecycle(t *testing.T) {
	t.Parallel()
	p := NewPIIDetector()
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := p.Health(context.Background()); err != nil {
		t.Fatalf("health: %v", err)
	}
	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestInjectionDetectorLifecycle(t *testing.T) {
	t.Parallel()
	d := NewInjectionDetector()
	if err := d.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := d.Health(context.Background()); err != nil {
		t.Fatalf("health: %v", err)
	}
	if err := d.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestRegisterAttachesPlugins(t *testing.T) {
	t.Parallel()
	s := supervisorForTest()
	Register(s)
	got := s.List()
	if len(got) != 2 {
		t.Fatalf("expected 2 registered, got %d (%v)", len(got), got)
	}
	want := map[string]bool{"pii-redactor": false, "prompt-injection": false}
	for _, n := range got {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("expected %q to be registered", name)
		}
	}
}
