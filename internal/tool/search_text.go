package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jgerontis/styx/internal/provider"
)

const defaultTextSearchResults = 50

// SearchText finds literal text matches inside workspace files.
type SearchText struct {
	root string
}

// NewSearchText creates a literal text search tool restricted to root.
func NewSearchText(root string) *SearchText {
	return &SearchText{root: root}
}

// Definition returns the search_text function schema.
func (t *SearchText) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "search_text",
		Description: "Find literal text in workspace files. Use for names, strings, config keys, filenames, and documentation; returns file paths and line numbers.",
		Schema:      json.RawMessage(`{"type":"object","required":["query"],"properties":{"query":{"type":"string","description":"Literal text to find"},"path":{"type":"string","description":"Optional workspace-relative directory or file to search"},"max_results":{"type":"integer","minimum":1,"maximum":200,"description":"Maximum matches to return"}}}`),
	}
}

// Execute searches workspace files for literal text.
func (t *SearchText) Execute(_ context.Context, args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("search_text requires a non-empty query")
	}
	path, _ := args["path"].(string)
	root, err := t.resolve(path)
	if err != nil {
		return "", err
	}
	maxResults := numberArg(args, "max_results", defaultTextSearchResults)
	if maxResults < 1 || maxResults > 200 {
		return "", fmt.Errorf("max_results must be between 1 and 200")
	}

	var matches []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "bin" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if len(matches) >= maxResults {
			return fs.SkipAll
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		relative, err := filepath.Rel(t.root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		for line, text := range strings.Split(string(data), "\n") {
			if strings.Contains(text, query) {
				matches = append(matches, fmt.Sprintf("%s:%d: %s", relative, line+1, text))
				if len(matches) >= maxResults {
					return fs.SkipAll
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "No matches found. search_text is case-sensitive; if unsure of exact casing, read the file directly instead of guessing variations.", nil
	}
	return strings.Join(matches, "\n"), nil
}

func (t *SearchText) resolve(path string) (string, error) {
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
