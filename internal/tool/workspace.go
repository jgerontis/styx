package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WorkspaceSummary is a compact description of the active project.
type WorkspaceSummary struct {
	Root        string
	ProjectName string
	Description string
}

// InspectWorkspace gathers stable, read-only repository metadata.
func InspectWorkspace(root string) (WorkspaceSummary, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return WorkspaceSummary{}, fmt.Errorf("resolve workspace root: %w", err)
	}
	absRoot, err = findWorkspaceRoot(absRoot)
	if err != nil {
		return WorkspaceSummary{}, err
	}

	summary := WorkspaceSummary{
		Root:        absRoot,
		ProjectName: filepath.Base(absRoot),
	}
	readme, err := os.ReadFile(filepath.Join(absRoot, "README.md"))
	if err == nil {
		summary.Description = firstParagraph(string(readme))
	}

	return summary, nil
}

func findWorkspaceRoot(start string) (string, error) {
	for dir := start; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start, nil
		}
	}
}

func firstParagraph(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var paragraph []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(paragraph) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		paragraph = append(paragraph, line)
	}

	return strings.Join(paragraph, " ")
}
