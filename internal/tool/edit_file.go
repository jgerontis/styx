package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jgerontis/styx/internal/provider"
)

// EditFile applies hash-anchored operations inside a workspace file.
type EditFile struct {
	root string
}

// NewEditFile creates an edit tool restricted to root.
func NewEditFile(root string) *EditFile {
	return &EditFile{root: root}
}

// Definition returns the edit_file function schema.
func (t *EditFile) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "edit_file",
		Description: "Apply an atomic batch of edits using fresh line:hash anchors from read_file. Operations are replace, delete, insert_before, or insert_after. First call with apply=false to preview the complete patch; call again with apply=true only after user approval. Any stale anchor rejects the entire patch.",
		Schema:      json.RawMessage(`{"type":"object","required":["path","operations","apply"],"properties":{"path":{"type":"string","description":"Workspace-relative file path"},"operations":{"type":"array","minItems":1,"description":"All edits for this file, applied atomically","items":{"type":"object","required":["kind","anchor"],"properties":{"kind":{"type":"string","enum":["replace","delete","insert_before","insert_after"]},"anchor":{"type":"string","description":"Fresh line:hash anchor from read_file, for example '12:1a2b3c4d'"},"content":{"type":"string","description":"Required for replace and insert operations; may contain multiple lines"}}}},"apply":{"type":"boolean","description":"false previews the complete patch; true applies it after approval"}}}`),
	}
}

type editOperation struct {
	Kind    string
	Anchor  string
	Content string
	Line    int
}

// Execute previews or atomically applies hash-anchored operations.
func (t *EditFile) Execute(_ context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("edit_file requires a non-empty path")
	}
	operations, err := parseOperations(args["operations"])
	if err != nil {
		return "", err
	}
	apply, ok := args["apply"].(bool)
	if !ok {
		return "", fmt.Errorf("edit_file requires apply to be true or false")
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
	preview, err := validateAndPreview(path, lines, operations)
	if err != nil {
		return "", err
	}
	if !apply {
		return "Preview:\n" + preview, nil
	}
	lines = applyOperations(lines, operations)
	mode := os.FileMode(0o644)
	if info, err := os.Stat(resolved); err == nil {
		mode = info.Mode()
	}
	if err := writeAtomically(resolved, strings.Join(lines, "\n"), mode); err != nil {
		return "", err
	}
	return "Applied:\n" + preview, nil
}

func parseOperations(value interface{}) ([]editOperation, error) {
	values, ok := value.([]interface{})
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("edit_file requires at least one operation")
	}
	operations := make([]editOperation, 0, len(values))
	seenLines := make(map[int]bool)
	for _, value := range values {
		raw, ok := value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("edit_file operations must be objects")
		}
		kind, _ := raw["kind"].(string)
		anchor, _ := raw["anchor"].(string)
		line, _, err := parseAnchor(anchor)
		if err != nil {
			return nil, err
		}
		if kind != "replace" && kind != "delete" && kind != "insert_before" && kind != "insert_after" {
			return nil, fmt.Errorf("invalid edit operation %q", kind)
		}
		content, _ := raw["content"].(string)
		if (kind == "replace" || kind == "insert_before" || kind == "insert_after") && content == "" {
			return nil, fmt.Errorf("edit operation %q requires content", kind)
		}
		if seenLines[line] {
			return nil, fmt.Errorf("multiple edit operations target line %d", line)
		}
		seenLines[line] = true
		operations = append(operations, editOperation{Kind: kind, Anchor: anchor, Content: content, Line: line})
	}
	return operations, nil
}

func validateAndPreview(path string, lines []string, operations []editOperation) (string, error) {
	var preview strings.Builder
	for _, operation := range operations {
		line, expectedHash, _ := parseAnchor(operation.Anchor)
		if line < 1 || line > len(lines) {
			return "", fmt.Errorf("anchor line %d is outside %s", line, path)
		}
		oldContent := lines[line-1]
		if lineHash(oldContent) != expectedHash {
			return "", fmt.Errorf("stale anchor %q: line %d changed; re-read %s", operation.Anchor, line, path)
		}
		fmt.Fprintf(&preview, "%s:%d\n", path, line)
		switch operation.Kind {
		case "replace":
			fmt.Fprintf(&preview, "- %s\n+ %s\n", oldContent, operation.Content)
		case "delete":
			fmt.Fprintf(&preview, "- %s\n", oldContent)
		case "insert_before":
			fmt.Fprintf(&preview, "+ %s\n  %s\n", operation.Content, oldContent)
		case "insert_after":
			fmt.Fprintf(&preview, "  %s\n+ %s\n", oldContent, operation.Content)
		}
	}
	return strings.TrimSuffix(preview.String(), "\n"), nil
}

func applyOperations(lines []string, operations []editOperation) []string {
	sort.Slice(operations, func(i, j int) bool { return operations[i].Line > operations[j].Line })
	for _, operation := range operations {
		index := operation.Line - 1
		content := strings.Split(operation.Content, "\n")
		switch operation.Kind {
		case "replace":
			lines = append(lines[:index], append(content, lines[index+1:]...)...)
		case "delete":
			lines = append(lines[:index], lines[index+1:]...)
		case "insert_before":
			lines = append(lines[:index], append(content, lines[index:]...)...)
		case "insert_after":
			lines = append(lines[:index+1], append(content, lines[index+1:]...)...)
		}
	}
	return lines
}

func parseAnchor(anchor string) (int, string, error) {
	parts := strings.Split(anchor, ":")
	if len(parts) != 2 || parts[1] == "" {
		return 0, "", fmt.Errorf("invalid anchor %q; expected line:hash", anchor)
	}
	line, err := strconv.Atoi(parts[0])
	if err != nil || line < 1 {
		return 0, "", fmt.Errorf("invalid anchor %q; expected line:hash", anchor)
	}
	return line, parts[1], nil
}

func (t *EditFile) resolve(path string) (string, error) {
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

func writeAtomically(path, content string, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".styx-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.WriteString(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
