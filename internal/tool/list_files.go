package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jgerontis/styx/internal/provider"
)

const defaultListFilesDepth = 2
const defaultListFilesResults = 100

// ListFiles returns a bounded file tree for a workspace path.
type ListFiles struct {
	root string
}

// NewListFiles creates a file listing tool restricted to root.
func NewListFiles(root string) *ListFiles {
	return &ListFiles{root: root}
}

// Definition returns the list_files function schema.
func (t *ListFiles) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "list_files",
		Description: "List files and directories in the workspace without reading file contents. Use this to understand an unfamiliar repository layout before searching or reading files.",
		Schema:      json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Optional workspace-relative directory"},"max_depth":{"type":"integer","minimum":0,"maximum":10,"description":"Maximum directory depth, default 2"},"max_results":{"type":"integer","minimum":1,"maximum":500,"description":"Maximum entries to return, default 100"}}}`),
	}
}

// Execute returns relative workspace entries subject to depth and result limits.
func (t *ListFiles) Execute(_ context.Context, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	root, err := t.resolve(path)
	if err != nil {
		return "", err
	}
	maxDepth := numberArg(args, "max_depth", defaultListFilesDepth)
	maxResults := numberArg(args, "max_results", defaultListFilesResults)
	if maxDepth < 0 || maxDepth > 10 {
		return "", fmt.Errorf("max_depth must be between 0 and 10")
	}
	if maxResults < 1 || maxResults > 500 {
		return "", fmt.Errorf("max_results must be between 1 and 500")
	}

	var entries []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relativeToStart, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		depth := len(strings.Split(relativeToStart, string(filepath.Separator)))
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "bin" || entry.Name() == "vendor" || depth > maxDepth) {
			return filepath.SkipDir
		}
		if depth > maxDepth {
			return nil
		}
		relativeToWorkspace, err := filepath.Rel(t.root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			relativeToWorkspace += "/"
		}
		entries = append(entries, relativeToWorkspace)
		if len(entries) >= maxResults {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "No files found.", nil
	}
	sort.Strings(entries)
	return strings.Join(entries, "\n"), nil
}

func (t *ListFiles) resolve(path string) (string, error) {
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
