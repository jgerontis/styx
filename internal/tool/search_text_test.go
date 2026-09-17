package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchTextReturnsLineNumberedMatches(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "example.go"), []byte("package example\nfunc Target() {}\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	result, err := NewSearchText(root).Execute(context.Background(), map[string]interface{}{"query": "Target"})
	if err != nil {
		t.Fatalf("search text: %v", err)
	}
	if result != "example.go:2: func Target() {}" {
		t.Errorf("result = %q", result)
	}
}

func TestSearchTextRespectsWorkspaceBoundary(t *testing.T) {
	_, err := NewSearchText(t.TempDir()).Execute(context.Background(), map[string]interface{}{"query": "secret", "path": ".."})
	if err == nil || !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("expected workspace boundary error, got %v", err)
	}
}
