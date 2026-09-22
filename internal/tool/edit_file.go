package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
		// A model that pasted old_string straight from read_file's "N: " output
		// still has a shot at a unique match once those prefixes are stripped.
		if stripped, ok := stripReadFileLinePrefixes(oldString); ok && strings.Count(content, stripped) == 1 {
			oldString = stripped
			count = 1
		}
	}
	if count == 0 {
		// Tolerate a model retyping a block with different leading/trailing
		// whitespace per line, as long as exactly one block in the file matches
		// once that whitespace drift is ignored.
		if matched, ok := matchIgnoringLineWhitespace(content, oldString); ok {
			oldString = matched
			count = 1
		}
	}
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

var readFileLinePrefix = regexp.MustCompile(`^\d+: `)

// stripReadFileLinePrefixes undoes read_file's "N: " numbering on every line,
// but only when every line carries it — otherwise this isn't that mistake.
func stripReadFileLinePrefixes(s string) (string, bool) {
	lines := strings.Split(s, "\n")
	stripped := make([]string, len(lines))
	matchedAny := false
	for i, line := range lines {
		if line == "" {
			stripped[i] = line
			continue
		}
		loc := readFileLinePrefix.FindStringIndex(line)
		if loc == nil {
			return s, false
		}
		stripped[i] = line[loc[1]:]
		matchedAny = true
	}
	if !matchedAny {
		return s, false
	}
	return strings.Join(stripped, "\n"), true
}

// matchIgnoringLineWhitespace looks for a block in content whose lines equal
// oldString's lines once each side is trimmed, tolerating whitespace drift a
// model introduces when retyping instead of copying exactly. It only returns
// a match when exactly one candidate block resolves to exactly one occurrence
// in content, so ambiguous drift never picks a side.
func matchIgnoringLineWhitespace(content, oldString string) (string, bool) {
	searchLines := strings.Split(oldString, "\n")
	if len(searchLines) > 0 && searchLines[len(searchLines)-1] == "" {
		searchLines = searchLines[:len(searchLines)-1]
	}
	if len(searchLines) == 0 {
		return "", false
	}
	contentLines := strings.Split(content, "\n")

	candidates := make(map[string]bool)
	for i := 0; i+len(searchLines) <= len(contentLines); i++ {
		matches := true
		for j, searchLine := range searchLines {
			if strings.TrimSpace(contentLines[i+j]) != strings.TrimSpace(searchLine) {
				matches = false
				break
			}
		}
		if matches {
			candidates[strings.Join(contentLines[i:i+len(searchLines)], "\n")] = true
		}
	}

	if len(candidates) != 1 {
		return "", false
	}
	for block := range candidates {
		if strings.Count(content, block) == 1 {
			return block, true
		}
	}
	return "", false
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
