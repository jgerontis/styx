package message

import "time"

// Role is the role of the message sender.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ContentPart is a part of message content (text, image, tool result).
type ContentPart struct {
	Type string      // "text", "image", "tool_result"
	Text string      // for text
	Data interface{} // for structured data
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID       string                 // unique call ID
	ToolName string                 // name of the tool
	Args     map[string]interface{} // arguments
}

// ToolResult is the result of executing a tool.
type ToolResult struct {
	ToolCallID string // ID of the ToolCall this responds to
	Content    string // result content
	Error      string // error message if tool failed
}

// Message represents a single turn in a conversation.
type Message struct {
	Role      Role
	Content   []ContentPart // v1: typically just one text part; v2: multimodal
	ToolCalls []ToolCall    // set on assistant role messages
	Timestamp time.Time
}

// String returns a text representation of content.
func (m *Message) String() string {
	if len(m.Content) == 0 {
		return ""
	}
	return m.Content[0].Text
}

// NewTextMessage creates a message with a single text part.
func NewTextMessage(role Role, text string) *Message {
	return &Message{
		Role: role,
		Content: []ContentPart{
			{Type: "text", Text: text},
		},
		Timestamp: time.Now(),
	}
}
