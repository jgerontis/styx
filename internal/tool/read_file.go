package tool

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jgerontis/styx/internal/provider"
)

// ReadFile reads a bounded range of a file inside a workspace.
type ReadFile struct {
	root string
}

// NewReadFile creates a read-only file tool restricted to root.
func NewReadFile(root string) *ReadFile {
	return &ReadFile{root: root}
}

// Definition returns the read_file function schema.
func (t *ReadFile) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "read_file",
		Description: "Read a text file in the workspace. Each line includes a line:hash anchor required by edit_file. Use start_line and end_line to request only the relevant range.",
		Schema:      json.RawMessage(`{"type":"object","required":["path"],"properties":{"path":{"type":"string","description":"Workspace-relative file path"},"start_line":{"type":"integer","minimum":1,"description":"First line to read, inclusive"},"end_line":{"type":"integer","minimum":1,"description":"Last line to read, inclusive"}}}`),
	}
}

// Execute reads the requested file range.
func (t *ReadFile) Execute(_ context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("read_file requires a non-empty path")
	}
	resolved, err := t.resolve(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	start := numberArg(args, "start_line", 1)
	end := numberArg(args, "end_line", len(lines))
	if start < 1 || end < start || start > len(lines) {
		return "", fmt.Errorf("invalid line range %d-%d for %s", start, end, path)
	}
	if end > len(lines) {
		end = len(lines)
	}

	var result strings.Builder
	for line := start; line <= end; line++ {
		fmt.Fprintf(&result, "%d:%s | %s\n", line, lineHash(lines[line-1]), lines[line-1])
	}
	return result.String(), nil
}

func lineHash(line string) string {
	digest := sha256.Sum256([]byte(line))
	return fmt.Sprintf("%x", digest[:4])
}

func (t *ReadFile) resolve(path string) (string, error) {
	return resolveWorkspacePath(t.root, path)
}

func resolveWorkspacePath(root, path string) (string, error) {
	resolved := filepath.Join(root, path)
	if filepath.IsAbs(path) {
		resolved = path
	}
	resolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside the workspace", path)
	}
	return resolved, nil
}

func numberArg(args map[string]interface{}, name string, fallback int) int {
	value, ok := args[name]
	if !ok {
		return fallback
	}
	switch value := value.(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return fallback
	}
}
