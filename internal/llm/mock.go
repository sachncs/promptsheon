package llm

import (
	"context"
	"sync"
	"time"

	"promptsheon/internal/models"
)

// Mock is a controllable provider for testing. It returns a configured
// response and records all calls for assertion.
type Mock struct {
	mu       sync.Mutex
	Response string
	Error    error
	Calls    []Request
}

// NewMock creates a Mock that returns the given content.
func NewMock(content string) *Mock {
	return &Mock{Response: content}
}

func (m *Mock) Name() string { return "mock" }

// Complete returns the configured response and records the call.
func (m *Mock) Complete(_ context.Context, req *Request) (*Response, error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, *req)
	m.mu.Unlock()

	if m.Error != nil {
		return nil, m.Error
	}

	return &Response{
		Content: m.Response,
		Usage: models.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
		Model:   req.Model,
		Latency: time.Millisecond,
	}, nil
}

// CallCount returns the number of calls made.
func (m *Mock) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Calls)
}

// LastCall returns the most recent request, or nil.
func (m *Mock) LastCall() *Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Calls) == 0 {
		return nil
	}
	r := m.Calls[len(m.Calls)-1]
	return &r
}
