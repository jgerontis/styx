package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jgerontis/styx/internal/message"
)

// TestProviderRegistry verifies provider registration and retrieval.
func TestProviderRegistry(t *testing.T) {
	reg := NewRegistry()

	// Create mock provider
	ollama := NewOllamaProvider("http://localhost:11434")

	// Register
	if err := reg.Register("ollama", ollama); err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	// Retrieve
	p, err := reg.Get("ollama")
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}

	if p.Name() != "ollama" {
		t.Errorf("expected name 'ollama', got %q", p.Name())
	}
}

// TestProviderCapabilities verifies Ollama capabilities.
func TestProviderCapabilities(t *testing.T) {
	ollama := NewOllamaProvider("http://localhost:11434")
	caps := ollama.Capabilities()

	if !caps.Streaming {
		t.Error("Ollama should support streaming")
	}
	if !caps.ToolCalling {
		t.Error("Ollama should support tool calling")
	}
}

// TestChatRequest builds a valid chat request.
func TestChatRequest(t *testing.T) {
	req := ChatRequest{
		Model: "llama2",
		Messages: []message.Message{
			*message.NewTextMessage(message.RoleUser, "Hello"),
		},
		Temperature: 0.7,
		MaxTokens:   100,
		Think:       false,
	}

	if req.Model != "llama2" {
		t.Errorf("expected model 'llama2', got %q", req.Model)
	}
	if len(req.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(req.Messages))
	}
}

func TestOllamaChatSendsThinkAndTokenLimit(t *testing.T) {
	var request map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		_, _ = w.Write([]byte("{\"done\":true}\n"))
	}))
	defer server.Close()

	stream, err := NewOllamaProvider(server.URL).Chat(context.Background(), ChatRequest{Model: "test", Think: false, MaxTokens: 123})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	defer stream.Close()
	if _, err := stream.Recv(); err != nil && err != io.EOF {
		t.Fatalf("read stream: %v", err)
	}
	if request["think"] != false {
		t.Errorf("think = %v, want false", request["think"])
	}
	if request["num_predict"] != float64(123) {
		t.Errorf("num_predict = %v, want 123", request["num_predict"])
	}
}
