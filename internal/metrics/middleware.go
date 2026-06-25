package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/sachn-cs/promptsheon/internal/trace"
)

// HTTPMiddleware instruments HTTP requests with metrics and tracing.
func HTTPMiddleware(collector *Collector, tracer trace.Tracer, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Skip metrics for liveness/readiness probes so the
			// request count is meaningful. (Cheap to do, common
			// monitoring pitfall.)
			isProbe := r.URL.Path == "/health" || r.URL.Path == "/ready"

			// Start trace span (if tracer available)
			var span *trace.Span
			if tracer != nil {
				span = tracer.Start(r.Context(), spanOperationName(r))
				span.SetAttribute("http.method", r.Method)
				// Strip query string from the recorded URL to avoid
				// persisting secrets passed in the URL (e.g. an
				// operator using ?api_key=... as a fallback).
				span.SetAttribute("http.url", pathOnly(r.URL))
			}

			// Wrap response writer to capture status
			rw := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rw, r)

			latency := time.Since(start)
			latencySec := DurationSec(latency)

			if collector != nil && !isProbe {
				collector.RequestsTotal.Inc()
				collector.RequestDuration.Observe(latencySec)
				if rw.status >= 400 {
					collector.ErrorsTotal.Inc()
				}
			}

			if span != nil {
				span.SetAttribute("http.status", fmt.Sprintf("%d", rw.status))
				span.SetAttribute("http.latency_ms", fmt.Sprintf("%d", latency.Milliseconds()))
				if rw.status >= 500 {
					span.SetError(fmt.Errorf("HTTP %d", rw.status))
				}
				span.Finish()
				if err := tracer.Finish(span); err != nil && logger != nil {
					logger.Error("failed to write trace span", "err", err)
				}
			}
		})
	}
}

// pathOnly returns the URL without the query string. Used to keep
// secrets out of trace attributes.
func pathOnly(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.Path
}

// spanOperationName returns the operation name for the span. It
// prefers the matched route pattern (no IDs in the path) and falls
// back to the raw method+path.
func spanOperationName(r *http.Request) string {
	if tmpl, ok := matchedRoute(r); ok && tmpl != "" {
		return r.Method + " " + tmpl
	}
	return r.Method + " " + r.URL.Path
}

// matchedRoute returns the mux template that matched r, if any.
// Returns false when the request was not served by a mux that
// populates the pattern (e.g. httptest without a mux).
func matchedRoute(r *http.Request) (string, bool) {
	if r == nil {
		return "", false
	}
	tmpl := r.Pattern
	if tmpl == "" {
		return "", false
	}
	return tmpl, true
}

// LLMMiddleware instruments LLM calls with metrics and tracing.
func LLMMiddleware(collector *Collector, tracer trace.Tracer, logger *slog.Logger) func(next LLMMiddlewareFunc) LLMMiddlewareFunc {
	return func(next LLMMiddlewareFunc) LLMMiddlewareFunc {
		return func(operation string, req any) (any, error) {
			start := time.Now()

			span := tracer.Start(context.Background(), "llm."+operation)
			span.SetAttribute("llm.operation", operation)

			resp, err := next(operation, req)

			latency := time.Since(start)
			latencySec := DurationSec(latency)

			collector.LLMCallsTotal.Inc()
			collector.LLMLatency.Observe(latencySec)

			if err != nil {
				span.SetError(err)
			}
			span.SetAttribute("llm.latency_ms", fmt.Sprintf("%d", latency.Milliseconds()))
			span.Finish()
			tracer.Finish(span) //nolint:errcheck

			return resp, err
		}
	}
}

// LLMMiddlewareFunc is the function signature for instrumented LLM calls.
type LLMMiddlewareFunc func(operation string, req any) (any, error)

// WorkflowMiddleware instruments workflow executions.
func WorkflowMiddleware(collector *Collector, tracer trace.Tracer) func(next WorkflowFunc) WorkflowFunc {
	return func(next WorkflowFunc) WorkflowFunc {
		return func(agentID string, input map[string]any) (map[string]any, error) {
			start := time.Now()
			collector.WorkflowActive.Inc()

			span := tracer.Start(context.Background(), "workflow.execute")
			span.SetAttribute("workflow.agent_id", agentID)

			output, err := next(agentID, input)

			latency := time.Since(start)
			collector.WorkflowRunsTotal.Inc()
			collector.WorkflowDuration.Observe(DurationSec(latency))
			collector.WorkflowActive.Dec()

			span.SetAttribute("workflow.latency_ms", fmt.Sprintf("%d", latency.Milliseconds()))
			if err != nil {
				span.SetError(err)
			}
			span.Finish()
			tracer.Finish(span) //nolint:errcheck

			return output, err
		}
	}
}

// WorkflowFunc is the function signature for instrumented workflow runs.
type WorkflowFunc func(agentID string, input map[string]any) (map[string]any, error)

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}
