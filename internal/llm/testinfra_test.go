// Shared test infrastructure for the LLM gateway property tests.
// This file is *test-only* (suffixed _test.go) and exposes a
// small httptest-backed Provider so the property harness can
// exercise the LLM stack end-to-end without spinning up a real
// upstream.
package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
)

// scriptedTransport is a minimal httptest-backed Provider. The
// provider POSTs a JSON-encoded Request to its server and
// unmarshals a JSON-encoded Response. The handler is provided
// by the test that owns the transport; the transport only does
// the JSON round-trip and surfaces transient HTTP failures as
// ErrTransient so the retry/property tests can inspect them.
type scriptedTransport struct {
	name   string
	server *httptest.Server
}

// Name returns the scripted provider name.
func (s *scriptedTransport) Name() string { return s.name }

// Complete POSTs the request to the canned server and returns
// the response. The wrapped HTTP round-trip is the property
// test's "spec" — every byte that the production HTTP transport
// would see, our scripted handler sees too.
func (s *scriptedTransport) Complete(ctx context.Context, req *Request) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.server.URL, bytesReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, &ErrTransient{Cause: transientHTTP{status: resp.StatusCode, body: string(raw)}}
	}
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// transientHTTP is a minimal error surface for the scripted
// transport. We only need the status code and body for the
// property tests to inspect.
type transientHTTP struct {
	status int
	body   string
}

func (t transientHTTP) Error() string { return t.body }

// bytesReader returns an io.Reader over b. It exists so the
// provider file does not need to import bytes for one trivial
// helper.
func bytesReader(b []byte) io.Reader { return &sliceReader{b: b} }

type sliceReader struct {
	b []byte
	i int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
