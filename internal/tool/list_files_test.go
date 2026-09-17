package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListFilesRespectsDepthAndSkipsGeneratedDirectories(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"README.md", "internal/tool.go", "internal/deep/file.go", ".git/config", "bin/styx"} {
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("fixture"), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}

	result, err := NewListFiles(root).Execute(context.Background(), map[string]interface{}{"max_depth": float64(2)})
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	for _, expected := range []string{"README.md", "internal/", "internal/tool.go"} {
		if !strings.Contains(result, expected) {
			t.Errorf("expected %q in %q", expected, result)
		}
	}
	for _, unexpected := range []string{"internal/deep/file.go", ".git/", "bin/"} {
		if strings.Contains(result, unexpected) {
			t.Errorf("did not expect %q in %q", unexpected, result)
		}
	}
}

func TestListFilesRejectsOutsideWorkspace(t *testing.T) {
	_, err := NewListFiles(t.TempDir()).Execute(context.Background(), map[string]interface{}{"path": ".."})
	if err == nil || !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("expected workspace boundary error, got %v", err)
	}
}
