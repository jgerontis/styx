package permission

// Decision determines whether a tool can run without user confirmation.
type Decision int

const (
	Allow Decision = iota
	Prompt
)

// Policy maps tools to their approval requirement.
type Policy struct{}

// Check returns the decision for a tool invocation.
func (Policy) Check(toolName string) Decision {
	switch toolName {
	case "list_files", "read_file", "search_text", "search_structure":
		return Allow
	default:
		return Prompt
	}
}
