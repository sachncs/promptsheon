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

	"github.com/sachncs/promptsheon/internal/injection"
	"github.com/sachncs/promptsheon/internal/redactor"
	"github.com/sachncs/promptsheon/internal/supervisor"
)

// PIIDetector adapts redactor.Redactor to the supervisor.Plugin
// interface. The redactor's real work happens at the request path
// via Redactor.CheckGuardrail.
type PIIDetector struct{ R *redactor.Redactor }

func NewPIIDetector() *PIIDetector { return &PIIDetector{R: redactor.NewRedactor()} }

func (p *PIIDetector) Start(context.Context) error  { return nil }
func (p *PIIDetector) Stop(context.Context) error   { return nil }
func (p *PIIDetector) Health(context.Context) error { return nil }

// InjectionDetector adapts injection.Detector.
type InjectionDetector struct{ D *injection.Detector }

func NewInjectionDetector() *InjectionDetector {
	return &InjectionDetector{D: injection.NewDetector()}
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
