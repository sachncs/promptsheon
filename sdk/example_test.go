package sdk_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/sachncs/promptsheon/sdk"
)

// ExampleNew demonstrates the canonical way to construct a
// Client. The base URL points at a Promptsheon daemon; the API
// key comes from the workspace's settings page.
func ExampleNew() {
	client := sdk.New("http://localhost:8080", "psn_example_key")
	health, err := client.Health(context.Background())
	if err != nil {
		fmt.Println("health error:", err)
		return
	}
	fmt.Println("status:", health.Status)
}

// ExampleListProviders exercises the providers endpoint using a
// httptest server that returns a small canned response. The
// pattern is useful in downstream tests where you want to
// exercise the SDK without spinning up a real daemon.
func ExampleClient_ListProviders() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"providers":["openai","anthropic"]}`)
	}))
	defer ts.Close()

	client := sdk.New(ts.URL, "test-key")
	providers, err := client.ListProviders(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("providers:", providers)
}

// ExampleClient_Health shows the smallest end-to-end SDK call:
// a health check. It is the recommended first call when wiring
// the SDK into a new application so transient startup errors
// surface fast.
func ExampleClient_Health() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer ts.Close()

	client := sdk.New(ts.URL, "test-key")
	health, err := client.Health(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("status:", health.Status)
}
