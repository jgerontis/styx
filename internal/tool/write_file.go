package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jgerontis/styx/internal/provider"
)

// WriteFile creates a new workspace file atomically.
type WriteFile struct{ root string }

func NewWriteFile(root string) *WriteFile { return &WriteFile{root: root} }

func (t *WriteFile) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{Name: "write_file", Description: "Create a new file in the workspace. Refuses to overwrite an existing file. First call with apply=false to preview, then apply=true after user approval.", Schema: json.RawMessage(`{"type":"object","required":["path","content","apply"],"properties":{"path":{"type":"string"},"content":{"type":"string"},"apply":{"type":"boolean"}}}`)}
}

func (t *WriteFile) Execute(_ context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("write_file requires a non-empty path")
	}
	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("write_file requires content")
	}
	apply, ok := args["apply"].(bool)
	if !ok {
		return "", fmt.Errorf("write_file requires apply to be true or false")
	}
	resolved, err := resolveWorkspacePath(t.root, path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(resolved); err == nil {
		return "", fmt.Errorf("refusing to overwrite existing file %s", path)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	preview := fmt.Sprintf("%s\n+ %s", path, strings.ReplaceAll(content, "\n", "\n+ "))
	if !apply {
		return "Preview:\n" + preview, nil
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return "", err
	}
	if err := writeAtomically(resolved, content, 0o644); err != nil {
		return "", err
	}
	return "Created:\n" + preview, nil
}
