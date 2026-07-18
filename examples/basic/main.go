// Package main provides an example Promptsheon application.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/sachncs/promptsheon/sdk"
)

const sampleHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func main() {
	client := sdk.New("http://localhost:8080", "your-api-key-here")
	ctx := context.Background()

	// Server health.
	health, err := client.Health(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Server status: %s\n", health.Status)

	// Assume the workspace + project + capability + version already exist
	// (curl created them in the README quickstart). Drive the Release +
	// Approval lifecycle through the SDK.
	const versionID = "v1"
	const releaseVoter = "alice"

	rel, err := client.CreateRelease(ctx, versionID, sdk.CreateReleaseRequest{Environment: "prod"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("release=%s status=%s\n", rel.ID, rel.Status)

	if _, err := client.Vote(ctx, rel.ID, sdk.VoteRequest{Identity: releaseVoter, Decision: "approve"}); err != nil {
		log.Fatal(err)
	}
	if _, err := client.Activate(ctx, rel.ID); err != nil {
		log.Fatal(err)
	}
	out, err := client.Invoke(ctx, rel.ID, sdk.InvokeRequest{
		Inputs: map[string]any{"q": "hello"},
		Model:  "claude-opus-4",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("invoked: %s\n", out.ID)
}

// keep sampleHash referenced so vet stays quiet when example is run.
var _ = sampleHash
