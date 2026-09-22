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

// ReadFile reads a bounded range of a file inside a workspace.
type ReadFile struct {
	root string
}

const (
	defaultReadFileLineLimit = 2000
	maxReadFileLineLength    = 2000
	maxReadFileBytes         = 50 * 1024
)

// NewReadFile creates a read-only file tool restricted to root.
func NewReadFile(root string) *ReadFile {
	return &ReadFile{root: root}
}

// Definition returns the read_file function schema.
func (t *ReadFile) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "read_file",
		Description: "Read a text file or line range from the workspace. Copy exact text after the line prefix into edit_file old_string; request the smallest useful range.",
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
	lineCapped := false
	if end-start+1 > defaultReadFileLineLimit {
		end = start + defaultReadFileLineLimit - 1
		lineCapped = true
	}

	var result strings.Builder
	totalBytes := 0
	byteCapped := false
	last := start - 1
	for line := start; line <= end; line++ {
		text := lines[line-1]
		if len(text) > maxReadFileLineLength {
			text = text[:maxReadFileLineLength] + fmt.Sprintf("... (line truncated to %d chars)", maxReadFileLineLength)
		}
		entry := fmt.Sprintf("%d: %s\n", line, text)
		if totalBytes+len(entry) > maxReadFileBytes {
			byteCapped = true
			break
		}
		result.WriteString(entry)
		totalBytes += len(entry)
		last = line
	}
	if byteCapped {
		fmt.Fprintf(&result, "(Output capped at %dKB. Showing lines %d-%d. Use start_line=%d to continue.)\n", maxReadFileBytes/1024, start, last, last+1)
	} else if lineCapped {
		fmt.Fprintf(&result, "(Showing lines %d-%d of %d. Use start_line=%d to continue.)\n", start, last, len(lines), last+1)
	}
	return result.String(), nil
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
