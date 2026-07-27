// Package builtins registers the in-process built-in Guardrail
// plugins with the supervisor. The two built-ins shipped today:
//
//   - redactor (internal/redactor): PII redaction
//   - injection (internal/injection): prompt-injection heuristic
//
// Both run as in-process plugins through the supervisor. The
// remote-plugin path (gRPC over UDS) ships in a follow-on.
package builtins

import (
	"context"
	"time"

	"github.com/sachncs/promptsheon/backend"
	"github.com/sachncs/promptsheon/backend/supervisor"
)

// PIIDetector adapts backend.Redactor to the supervisor.Plugin
// interface. The redactor's real work happens at the request path
// via Redactor.CheckGuardrail.
type PIIDetector struct{ R *backend.Redactor }

func NewPIIDetector() *PIIDetector { return &PIIDetector{R: backend.NewRedactor()} }

func (p *PIIDetector) Start(context.Context) error  { return nil }
func (p *PIIDetector) Stop(context.Context) error   { return nil }
func (p *PIIDetector) Health(context.Context) error { return nil }

// InjectionDetector adapts backend.Detector.
type InjectionDetector struct{ D *backend.Detector }

func NewInjectionDetector() *InjectionDetector {
	return &InjectionDetector{D: backend.NewDetector()}
}

func (p *InjectionDetector) Start(context.Context) error  { return nil }
func (p *InjectionDetector) Stop(context.Context) error   { return nil }
func (p *InjectionDetector) Health(context.Context) error { return nil }

// Register attaches every built-in to the supervisor with a
// sensible RestartPolicy: 3 restarts max with exponential
// backoff up to 30 seconds. Ops can override per-plugin later.
func Register(s *supervisor.Supervisor) {
	s.Register("pii-redactor", NewPIIDetector(), defaultPolicy())
	s.Register("prompt-injection", NewInjectionDetector(), defaultPolicy())
}

func defaultPolicy() supervisor.RestartPolicy {
	return supervisor.RestartPolicy{
		MaxRestarts: 3,
		Backoff:     time.Second,
		MaxBackoff:  30 * time.Second,
	}
}
