package provider

import (
	"context"
)

// Provider abstracts LLM backends (Ollama, OpenAI, etc.)
type Provider interface {
	// Name returns the provider name (e.g., "ollama", "openai")
	Name() string

	// Models returns available models for this provider
	Models(ctx context.Context) ([]Model, error)

	// Chat sends a request to the LLM and returns a stream of deltas
	Chat(ctx context.Context, req ChatRequest) (StreamReader, error)

	// Capabilities returns what this provider supports
	Capabilities() Capabilities

	// Connect establishes a connection to the provider
	Connect(ctx context.Context) error

	// Disconnect closes the connection
	Disconnect(ctx context.Context) error
}
