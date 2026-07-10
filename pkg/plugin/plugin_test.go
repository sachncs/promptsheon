package plugin

import (
	"context"
	"errors"
	"testing"
)

func TestValidateDescriptorRequiresName(t *testing.T) {
	t.Parallel()
	if err := validateDescriptor(PluginDescriptor{Services: []string{"p"}}, []string{"p"}); err == nil {
		t.Fatalf("expected error for empty name")
	}
}

func TestValidateDescriptorServiceMatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		declared []string
		expected []string
		wantErr  bool
	}{
		{"match", []string{"a", "b"}, []string{"a"}, false},
		{"missing", []string{"a"}, []string{"a", "b"}, true},
		{"empty declared", nil, []string{"a"}, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := PluginDescriptor{Name: "p", Services: tc.declared}
			err := validateDescriptor(d, tc.expected)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestErrorsIsCorrect(t *testing.T) {
	t.Parallel()
	if !errors.Is(ErrServiceNotDeclared, ErrServiceNotDeclared) {
		t.Fatalf("errors.Is on ErrServiceNotDeclared")
	}
	if !errors.Is(ErrVersionTooOld, ErrVersionTooOld) {
		t.Fatalf("errors.Is on ErrVersionTooOld")
	}
	if errors.Is(ErrServiceNotDeclared, ErrVersionTooOld) {
		t.Fatalf("sentinels must be distinct")
	}
}

type fakePlugin struct {
	d PluginDescriptor
}

func (f *fakePlugin) Handshake(_ context.Context) (PluginDescriptor, error) { return f.d, nil }
func (f *fakePlugin) Shutdown(_ context.Context) error                      { return nil }

func TestPluginInterfaceConformance(t *testing.T) {
	t.Parallel()
	var _ Plugin = (*fakePlugin)(nil)
}
