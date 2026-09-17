package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jgerontis/styx/internal/provider"
)

// SearchStructure finds structural code matches with ast-grep.
type SearchStructure struct {
	root   string
	binary string
}

// NewSearchStructure creates an ast-grep search tool restricted to root.
func NewSearchStructure(root string) *SearchStructure {
	return &SearchStructure{root: root, binary: "ast-grep"}
}

// Definition returns the search_structure function schema.
func (t *SearchStructure) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "search_structure",
		Description: "Find code by syntax with ast-grep. Use for declarations, imports, calls, and components. Patterns must be valid code in the requested language; $NAME matches one node and $$$NODES matches many.",
		Schema:      json.RawMessage(`{"type":"object","required":["pattern","language"],"properties":{"pattern":{"type":"string","description":"Valid code-shaped ast-grep pattern. Examples: Go function: 'func $NAME($$$PARAMS) { $$$BODY }'; TypeScript call: '$FUNC($$$ARGS)'; import: 'import $NAME from $SOURCE'"},"language":{"type":"string","description":"Required ast-grep language inferred from target files, such as go, ts, tsx, javascript, python, or rust"},"path":{"type":"string","description":"Optional workspace-relative file or directory to search"}}}`),
	}
}

// Execute searches code with ast-grep without invoking a shell.
func (t *SearchStructure) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return "", fmt.Errorf("search_structure requires a non-empty pattern")
	}
	language, ok := args["language"].(string)
	if !ok || language == "" {
		return "", fmt.Errorf("search_structure requires a language")
	}
	path, _ := args["path"].(string)
	root, err := t.resolve(path)
	if err != nil {
		return "", err
	}

	binary, err := t.findBinary()
	if err != nil {
		return "", fmt.Errorf("search_structure requires ast-grep; install it and ensure `ast-grep` is on PATH")
	}
	commandArgs := []string{"--pattern", pattern}
	commandArgs = append(commandArgs, "--lang", language)
	commandArgs = append(commandArgs, root)

	output, err := exec.CommandContext(ctx, binary, commandArgs...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ast-grep search: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if len(output) == 0 {
		return "No matches found.", nil
	}
	return string(output), nil
}

func (t *SearchStructure) findBinary() (string, error) {
	if t.binary != "ast-grep" {
		return exec.LookPath(t.binary)
	}
	if configured := os.Getenv("STYX_AST_GREP_PATH"); configured != "" {
		return exec.LookPath(configured)
	}
	return exec.LookPath(t.binary)
}

func (t *SearchStructure) resolve(path string) (string, error) {
	if path == "" {
		return t.root, nil
	}
	resolved := filepath.Join(t.root, path)
	if filepath.IsAbs(path) {
		resolved = path
	}
	resolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(t.root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside the workspace", path)
	}
	return resolved, nil
}
