package message

import (
	"testing"
	"time"
)

// TestNewTextMessage creates a text message.
func TestNewTextMessage(t *testing.T) {
	msg := NewTextMessage(RoleUser, "Hello, world")

	if msg.Role != RoleUser {
		t.Errorf("expected role %q, got %q", RoleUser, msg.Role)
	}
	if len(msg.Content) != 1 {
		t.Errorf("expected 1 content part, got %d", len(msg.Content))
	}
	if msg.Content[0].Text != "Hello, world" {
		t.Errorf("expected text 'Hello, world', got %q", msg.Content[0].Text)
	}
}

// TestMessageString returns the text representation.
func TestMessageString(t *testing.T) {
	msg := NewTextMessage(RoleAssistant, "Response")

	if msg.String() != "Response" {
		t.Errorf("expected 'Response', got %q", msg.String())
	}
}

// TestToolCall represents a tool invocation.
func TestToolCall(t *testing.T) {
	tc := ToolCall{
		ID:       "call-1",
		ToolName: "read_file",
		Args: map[string]interface{}{
			"file": "main.go",
		},
	}

	if tc.ToolName != "read_file" {
		t.Errorf("expected tool 'read_file', got %q", tc.ToolName)
	}
	if tc.Args["file"] != "main.go" {
		t.Errorf("expected file 'main.go', got %q", tc.Args["file"])
	}
}

// TestMessageWithToolCalls creates a message with tool calls.
func TestMessageWithToolCalls(t *testing.T) {
	msg := &Message{
		Role:      RoleAssistant,
		Content:   []ContentPart{{Type: "text", Text: "Analyzing..."}},
		ToolCalls: []ToolCall{{ID: "1", ToolName: "analyze"}},
		Timestamp: time.Now(),
	}

	if len(msg.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ToolName != "analyze" {
		t.Errorf("expected 'analyze', got %q", msg.ToolCalls[0].ToolName)
	}
}
