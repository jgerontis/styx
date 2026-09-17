package permission

import "strings"

// Decision determines whether a tool can run without user confirmation.
type Decision int

const (
	Allow Decision = iota
	Prompt
)

// Policy maps tools to their approval requirement.
type Policy struct {
	AllowedTools []string
}

// Check returns the decision for a tool invocation.
func (p Policy) Check(toolName string) Decision {
	for _, allowed := range p.AllowedTools {
		if allowed == toolName || (strings.HasSuffix(allowed, "*") && strings.HasPrefix(toolName, strings.TrimSuffix(allowed, "*"))) {
			return Allow
		}
	}
	switch toolName {
	case "list_files", "read_file", "search_text", "search_structure", "load_skill":
		return Allow
	default:
		return Prompt
	}
}
