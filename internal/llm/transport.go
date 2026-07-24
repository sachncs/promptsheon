package llm

import (
	"net"
	"net/http"
	"time"
)

// tunedTransport returns an http.Transport with the MaxIdleConns
// and IdleConnTimeout values that match the LLM gateway's
// benchmarks. PERF-LLM-1: the SDK default caps
// MaxIdleConnsPerHost at 2, which forces a new TCP+TLS handshake
// for almost every concurrent request to the upstream. The
// values here match the LLM gateway's expectations; the
// benchmark in internal/llm/benchmark_test.go pins the
// throughput at ~1500 RPS for an OpenAI-shaped payload.
//
// The transport is shared by the OpenAI and Anthropic clients
// through option.WithHTTPClient. Tests that need to inject a
// custom transport can wrap it via transportWrapper.
func tunedTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    false,
	}
}
