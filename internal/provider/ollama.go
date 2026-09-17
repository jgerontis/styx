package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OllamaProvider is an implementation of the Provider interface for Ollama
type OllamaProvider struct {
	baseURL   string
	client    *http.Client
	connected bool
}

// NewOllamaProvider creates a new Ollama provider instance.
func NewOllamaProvider(baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaProvider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{},
	}
}

// Name returns the provider name.
func (op *OllamaProvider) Name() string {
	return "ollama"
}

// Connect establishes a connection to Ollama.
func (op *OllamaProvider) Connect(ctx context.Context) error {
	// Verify connectivity by requesting available models
	_, err := op.Models(ctx)
	if err == nil {
		op.connected = true
	}
	return err
}

// Disconnect terminates the connection.
func (op *OllamaProvider) Disconnect(ctx context.Context) error {
	op.connected = false
	return nil
}

// Models returns available models from Ollama.
func (op *OllamaProvider) Models(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", op.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := op.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama API error: %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse models: %w", err)
	}

	models := make([]Model, len(result.Models))
	for i, m := range result.Models {
		models[i] = Model{
			ID:       m.Name,
			Provider: "ollama",
		}
	}

	return models, nil
}

// Chat sends a message to Ollama and streams the response.
func (op *OllamaProvider) Chat(ctx context.Context, req ChatRequest) (StreamReader, error) {
	// Build request body
	messages := make([]map[string]interface{}, len(req.Messages))
	for i, msg := range req.Messages {
		content := ""
		if len(msg.Content) > 0 {
			content = msg.Content[0].Text
		}
		messages[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": content,
		}
	}

	body := map[string]interface{}{
		"model":       req.Model,
		"messages":    messages,
		"stream":      true,
		"temperature": req.Temperature,
	}

	if req.MaxTokens > 0 {
		body["num_predict"] = req.MaxTokens
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", op.baseURL+"/api/chat", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := op.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send chat request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("ollama API error: %d", resp.StatusCode)
	}

	return &OllamaStreamReader{resp: resp}, nil
}

// Capabilities returns what Ollama supports.
func (op *OllamaProvider) Capabilities() Capabilities {
	return Capabilities{
		Streaming:   true,
		ToolCalling: false, // Ollama doesn't support function calling in base model
		Vision:      false,
		JSONMode:    false,
	}
}

// OllamaStreamReader reads streaming responses from Ollama.
type OllamaStreamReader struct {
	resp   *http.Response
	reader *bufio.Reader
}

// Recv reads the next delta from the stream.
func (osr *OllamaStreamReader) Recv() (Delta, error) {
	if osr.reader == nil {
		osr.reader = bufio.NewReader(osr.resp.Body)
	}

	line, err := osr.reader.ReadBytes('\n')
	if err != nil {
		if err == io.EOF {
			return Delta{Type: "done"}, io.EOF
		}
		return Delta{}, err
	}

	// Parse Ollama's streaming response format
	var msg struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Done  bool `json:"done"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(line, &msg); err != nil {
		return Delta{}, err
	}

	if msg.Message.Content != "" {
		return Delta{
			Type: "text",
			Text: msg.Message.Content,
		}, nil
	}

	if msg.Done {
		return Delta{
			Type: "done",
			Usage: &Usage{
				InputTokens:  msg.Usage.PromptTokens,
				OutputTokens: msg.Usage.CompletionTokens,
				TotalTokens:  msg.Usage.PromptTokens + msg.Usage.CompletionTokens,
			},
		}, nil
	}

	// Empty chunk; read next
	return osr.Recv()
}

// Close closes the response body.
func (osr *OllamaStreamReader) Close() error {
	if osr.resp != nil {
		return osr.resp.Body.Close()
	}
	return nil
}
