package tool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectWorkspaceReadsProjectMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("create git directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Example\n\nA compact project description.\n\nMore detail.\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	summary, err := InspectWorkspace(root)
	if err != nil {
		t.Fatalf("inspect workspace: %v", err)
	}
	if summary.ProjectName != filepath.Base(root) {
		t.Errorf("project name = %q, want %q", summary.ProjectName, filepath.Base(root))
	}
	if summary.Description != "A compact project description." {
		t.Errorf("description = %q", summary.Description)
	}
}

func TestInspectWorkspaceFindsRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("create git directory: %v", err)
	}
	nested := filepath.Join(root, "internal", "tool")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}

	summary, err := InspectWorkspace(nested)
	if err != nil {
		t.Fatalf("inspect workspace: %v", err)
	}
	if summary.Root != root {
		t.Errorf("workspace root = %q, want %q", summary.Root, root)
	}
}
