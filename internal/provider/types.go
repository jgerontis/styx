package provider

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/jgerontis/styx/internal/message"
)

// Model represents an LLM model with its parameters.
type Model struct {
	ID       string                 // e.g., "llama2", "gpt-4"
	Provider string                 // e.g., "ollama", "openai"
	Params   map[string]interface{} // model-specific params
}

// Capabilities describes what a provider supports.
type Capabilities struct {
	Streaming   bool
	ToolCalling bool
	Vision      bool
	JSONMode    bool
}

// ChatRequest is a request to send a message to the LLM.
type ChatRequest struct {
	Model       string
	Messages    []message.Message
	Tools       []ToolDefinition // available tools for function calling
	Temperature float32
	MaxTokens   int
	Think       bool
	// Provider-specific options
	Params map[string]interface{}
}

// ToolDefinition is the JSON Schema definition of a tool.
type ToolDefinition struct {
	Name        string          // tool name
	Description string          // what it does
	Schema      json.RawMessage // JSON Schema for input
}

// Delta is a single streamed chunk from the provider.
type Delta struct {
	Type      string         // "text" | "tool_call_start" | "tool_call_delta" | "tool_call_end" | "usage" | "error" | "done"
	Text      string         // for type="text"
	ToolCall  *ToolCallDelta // for tool call types
	Usage     *Usage         // for type="usage"
	Error     error          // for type="error"
	Timestamp int64          // Unix nano timestamp
}

// ToolCallDelta represents a partial or complete tool call.
type ToolCallDelta struct {
	ID       string
	ToolName string
	Args     json.RawMessage // accumulate partial JSON
	Done     bool            // true when call is complete
}

// Usage tracks token usage.
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// StreamReader reads deltas from a provider stream.
type StreamReader interface {
	Recv() (Delta, error) // io.EOF when done
	Close() error
}

// CollectResult collects all deltas from a stream into a message.
type CollectResult struct {
	Message *message.Message
	Usage   *Usage
	Error   error
}

// Collect reads all deltas from a StreamReader and assembles a complete message.
func Collect(sr StreamReader) (CollectResult, error) {
	defer sr.Close()

	var textParts []string
	var toolCalls []message.ToolCall
	var usage *Usage

	for {
		delta, err := sr.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return CollectResult{Error: err}, err
		}

		switch delta.Type {
		case "text":
			textParts = append(textParts, delta.Text)
		case "tool_call_start", "tool_call_delta", "tool_call_end":
			// Accumulate tool call (simplified; in reality would merge deltas)
			if delta.ToolCall != nil && delta.ToolCall.Done {
				var args map[string]interface{}
				_ = json.Unmarshal(delta.ToolCall.Args, &args)
				toolCalls = append(toolCalls, message.ToolCall{
					ID:       delta.ToolCall.ID,
					ToolName: delta.ToolCall.ToolName,
					Args:     args,
				})
			}
		case "usage":
			usage = delta.Usage
		case "error":
			return CollectResult{Error: delta.Error}, delta.Error
		}
	}

	msg := &message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentPart{
			{Type: "text", Text: fmt.Sprintf("%s", textParts)},
		},
		ToolCalls: toolCalls,
	}

	return CollectResult{Message: msg, Usage: usage}, nil
}
