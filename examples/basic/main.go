package main

import (
	"context"
	"fmt"
	"log"

	"promptsheon/sdk"
)

func main() {
	client := sdk.New("http://localhost:8080", "your-api-key-here")

	ctx := context.Background()

	// List all prompts
	prompts, err := client.ListPrompts(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d prompts\n", len(prompts))

	// Create a new prompt
	prompt, err := client.CreatePrompt(ctx, &sdk.CreatePromptRequest{
		Name:        "greeting",
		Content:     "You are a helpful assistant. Greet the user: {{name}}",
		Description: "A simple greeting prompt",
		Tags:        []string{"greeting", "assistant"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created prompt: %s (ID: %s)\n", prompt.Name, prompt.ID)

	// Run the prompt
	result, err := client.RunPrompt(ctx, prompt.ID, &sdk.RunPromptRequest{
		Variables: map[string]string{
			"name": "World",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %s\n", result.Content)

	// List available agents
	agents, err := client.ListAgents(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d agents\n", len(agents))

	// Health check
	health, err := client.Health(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Server status: %s\n", health.Status)
}
