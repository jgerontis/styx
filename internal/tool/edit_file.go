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
		Description: "Propose one exact text replacement in an existing workspace file. old_string must occur exactly once and include enough surrounding text to be unique. Styx previews and asks before applying.",
		Schema:      json.RawMessage(`{"type":"object","required":["path","old_string","new_string"],"properties":{"path":{"type":"string","description":"Workspace-relative file path"},"old_string":{"type":"string","description":"Exact existing text to replace, including whitespace and surrounding lines as needed to make it unique"},"new_string":{"type":"string","description":"Replacement text; may contain multiple lines"}}}`),
	}
}

// Execute previews or atomically applies an exact text replacement.
func (t *EditFile) Execute(_ context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("edit_file requires a non-empty path")
	}
	oldString, ok := args["old_string"].(string)
	if !ok || oldString == "" {
		return "", fmt.Errorf("edit_file requires a non-empty old_string")
	}
	newString, ok := args["new_string"].(string)
	if !ok {
		return "", fmt.Errorf("edit_file requires new_string")
	}
	if oldString == newString {
		return "", fmt.Errorf("old_string and new_string must differ")
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
	content := string(data)
	count := strings.Count(content, oldString)
	if count == 0 {
		return "", fmt.Errorf("old_string was not found in %s; re-read the file and copy the exact text", path)
	}
	if count > 1 {
		return "", fmt.Errorf("old_string appears %d times in %s; include more surrounding text to make it unique", count, path)
	}
	preview := fmt.Sprintf("%s\n- %s\n+ %s", path, oldString, newString)
	if !apply {
		return "Preview:\n" + preview, nil
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(resolved); err == nil {
		mode = info.Mode()
	}
	if err := writeAtomically(resolved, strings.Replace(content, oldString, newString, 1), mode); err != nil {
		return "", err
	}
	return "Applied:\n" + preview, nil
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
