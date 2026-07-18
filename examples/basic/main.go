// Package main provides an example Promptsheon application.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/sachncs/promptsheon/sdk"
)

func main() {
	client := sdk.New("http://localhost:8080", "your-api-key-here")
	ctx := context.Background()

	// Server health.
	health, err := client.Health(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Server status: %s\n", health.Status)

	// Drive the Release + Approval lifecycle through the SDK.
	// Workspace, project, capability, and version are assumed to
	// already exist (curl or CreateWorkspace/CreateCapability/AddVersion
	// from this SDK).
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
