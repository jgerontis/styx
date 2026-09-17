package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFilePreviewsThenCreatesNewFile(t *testing.T) {
	root := t.TempDir()
	tool := NewWriteFile(root)
	args := map[string]interface{}{"path": "nested/example.txt", "content": "new content\n", "apply": false}
	preview, err := tool.Execute(context.Background(), args)
	if err != nil || !strings.Contains(preview, "+ new content") {
		t.Fatalf("preview = %q, error = %v", preview, err)
	}
	args["apply"] = true
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("create file: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "nested", "example.txt"))
	if err != nil || string(data) != "new content\n" {
		t.Fatalf("file = %q, error = %v", data, err)
	}
}

func TestWriteFileRefusesOverwrite(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := NewWriteFile(root).Execute(context.Background(), map[string]interface{}{"path": "existing.txt", "content": "replacement", "apply": true})
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected overwrite refusal, got %v", err)
	}
}
