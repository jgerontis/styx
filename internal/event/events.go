package event

import (
	"time"
)

// Event is the interface for all events published on the bus.
type Event interface {
	isEvent()
}

// TokenEvent represents a streamed token from the provider.
type TokenEvent struct {
	Token     string
	Timestamp time.Time
}

func (TokenEvent) isEvent() {}

// ToolStartEvent signals the beginning of a tool execution.
type ToolStartEvent struct {
	ToolName  string
	Args      map[string]interface{}
	Timestamp time.Time
}

func (ToolStartEvent) isEvent() {}

// ToolEndEvent signals the completion of a tool execution.
type ToolEndEvent struct {
	ToolName  string
	Result    string
	Error     error
	Timestamp time.Time
}

func (ToolEndEvent) isEvent() {}

// ApprovalRequestEvent asks for user confirmation before executing a tool.
type ApprovalRequestEvent struct {
	ToolName  string
	Args      map[string]interface{}
	Reason    string
	Timestamp time.Time
}

func (ApprovalRequestEvent) isEvent() {}

// ApprovalResponseEvent is the user's response to an approval request.
type ApprovalResponseEvent struct {
	ToolName  string
	Approved  bool
	Timestamp time.Time
}

func (ApprovalResponseEvent) isEvent() {}

// System1DecisionEvent reports a fast typed decision made by the optional
// System 1 gate, so the CLI/TUI can surface it without waiting on System 2.
type System1DecisionEvent struct {
	ToolName   string
	Question   string
	Confidence float64
	Timestamp  time.Time
}

func (System1DecisionEvent) isEvent() {}

// DoneEvent signals completion of the agent's processing.
type DoneEvent struct {
	Timestamp time.Time
}

func (DoneEvent) isEvent() {}

// ErrorEvent represents an error during processing.
type ErrorEvent struct {
	Error     error
	Timestamp time.Time
}

func (ErrorEvent) isEvent() {}
