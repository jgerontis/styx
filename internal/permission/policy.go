package permission

import (
	"context"
	"strings"

	"github.com/jgerontis/styx/internal/system1"
)

// Decision determines whether a tool can run without user confirmation.
type Decision int

const (
	Allow Decision = iota
	Prompt
)

// DefaultGateThreshold is the confidence a System 1 Gate must clear before
// Evaluate will auto-allow a tool that static rules would otherwise prompt for.
const DefaultGateThreshold = 0.85

// Policy maps tools to their approval requirement.
type Policy struct {
	AllowedTools []string

	// Gate is an optional System 1 classifier consulted by Evaluate for
	// tools that aren't statically allowed. Nil means no gating: every
	// non-allow-listed tool prompts, exactly as Check behaves today.
	Gate system1.Gate
	// AutoApprove must be explicitly set for Gate to be able to turn a
	// Prompt into an Allow. Without it, Gate answers are ignored and
	// Evaluate behaves identically to Check.
	AutoApprove bool
	// GateThreshold overrides DefaultGateThreshold when non-zero.
	GateThreshold float64
}

// Check returns the static decision for a tool invocation, ignoring any
// configured System 1 Gate. Kept for callers that don't have tool arguments
// or a context on hand.
func (p Policy) Check(toolName string) Decision {
	for _, allowed := range p.AllowedTools {
		if allowed == toolName || (strings.HasSuffix(allowed, "*") && strings.HasPrefix(toolName, strings.TrimSuffix(allowed, "*"))) {
			return Allow
		}
	}
	switch toolName {
	case "list_files", "read_file", "search_text", "load_skill":
		return Allow
	default:
		return Prompt
	}
}

// Evaluate augments Check with an optional System 1 risk gate. It never
// weakens a static Allow, and only ever turns a Prompt into an Allow when
// AutoApprove is set and the Gate reports high-confidence safety; any Gate
// error, low confidence, or missing Gate fails safe to Prompt.
func (p Policy) Evaluate(ctx context.Context, toolName string, args map[string]interface{}) Decision {
	if p.Check(toolName) == Allow {
		return Allow
	}
	if p.Gate == nil || !p.AutoApprove {
		return Prompt
	}

	threshold := p.GateThreshold
	if threshold == 0 {
		threshold = DefaultGateThreshold
	}

	q := system1.Question{
		Name:         "safe",
		Primitive:    system1.Noul,
		Instructions: "Is this tool call safe to run without human approval (non-destructive, reversible, and scoped to the workspace)?",
	}
	answer, err := p.Gate.Answer(ctx, q, map[string]interface{}{"tool": toolName, "args": args})
	if err != nil || !answer.HighConfidence(threshold) {
		return Prompt
	}
	if answer.Noul >= threshold {
		return Allow
	}
	return Prompt
}
