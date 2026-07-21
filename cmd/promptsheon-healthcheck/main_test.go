package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestHealthCheckSucceeds(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}),
	}
	go srv.Serve(ln)
	defer srv.Shutdown(context.Background())

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	t.Setenv("PROMPTSHEON_HEALTHCHECK_PORT", portStr)
	t.Setenv("PROMPTSHEON_HEALTHCHECK_HOST", "127.0.0.1")

	if err := healthCheckOnce(); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestHealthCheckNon200Fails(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}),
	}
	go srv.Serve(ln)
	defer srv.Shutdown(context.Background())

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	t.Setenv("PROMPTSHEON_HEALTHCHECK_PORT", portStr)
	t.Setenv("PROMPTSHEON_HEALTHCHECK_HOST", "127.0.0.1")

	if err := healthCheckOnce(); err == nil {
		t.Fatal("expected non-nil error for 500 response")
	}
}

func healthCheckOnce() error {
	host := getenv("PROMPTSHEON_HEALTHCHECK_HOST", "localhost")
	portStr := getenv("PROMPTSHEON_HEALTHCHECK_PORT", "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://%s:%d/health", host, port)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
