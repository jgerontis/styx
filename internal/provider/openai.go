package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// OpenAIProvider is an implementation of the Provider interface for OpenAI API
type OpenAIProvider struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

// NewOpenAIProvider creates a new OpenAI provider instance.
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	return &OpenAIProvider{
		apiKey:  apiKey,
		client:  &http.Client{},
		baseURL: "https://api.openai.com/v1",
	}
}

// Name returns the provider name.
func (op *OpenAIProvider) Name() string {
	return "openai"
}

// Connect verifies the API key is valid.
func (op *OpenAIProvider) Connect(ctx context.Context) error {
	if op.apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY not set")
	}
	// Test connectivity by listing models
	_, err := op.Models(ctx)
	return err
}

// Disconnect is a no-op for OpenAI.
func (op *OpenAIProvider) Disconnect(ctx context.Context) error {
	return nil
}

// Models returns available models from OpenAI.
func (op *OpenAIProvider) Models(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", op.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+op.apiKey)

	resp, err := op.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error: %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse models: %w", err)
	}

	models := make([]Model, len(result.Data))
	for i, m := range result.Data {
		models[i] = Model{
			ID:       m.ID,
			Provider: "openai",
		}
	}

	return models, nil
}

// Chat sends a message to OpenAI and streams the response.
func (op *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (StreamReader, error) {
	// Build request messages
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
		body["max_tokens"] = req.MaxTokens
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", op.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+op.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := op.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send chat request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai API error: %d - %s", resp.StatusCode, string(body))
	}

	return &OpenAIStreamReader{resp: resp}, nil
}

// Capabilities returns what OpenAI supports.
func (op *OpenAIProvider) Capabilities() Capabilities {
	return Capabilities{
		Streaming:   true,
		ToolCalling: true,
		Vision:      true,
		JSONMode:    true,
	}
}

// OpenAIStreamReader reads streaming responses from OpenAI.
type OpenAIStreamReader struct {
	resp   *http.Response
	reader *bufio.Reader
}

// Recv reads the next delta from the stream.
func (osr *OpenAIStreamReader) Recv() (Delta, error) {
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

	// OpenAI uses Server-Sent Events format
	lineStr := strings.TrimSpace(string(line))
	if lineStr == "" || !strings.HasPrefix(lineStr, "data: ") {
		return osr.Recv() // Skip empty or non-data lines
	}

	data := strings.TrimPrefix(lineStr, "data: ")
	if data == "[DONE]" {
		return Delta{Type: "done"}, nil
	}

	var choice struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	}

	if err := json.Unmarshal([]byte(data), &choice); err != nil {
		return Delta{}, err
	}

	if choice.Delta.Content != "" {
		return Delta{
			Type: "text",
			Text: choice.Delta.Content,
		}, nil
	}

	// Empty chunk; read next
	return osr.Recv()
}

// Close closes the response body.
func (osr *OpenAIStreamReader) Close() error {
	if osr.resp != nil {
		return osr.resp.Body.Close()
	}
	return nil
}
