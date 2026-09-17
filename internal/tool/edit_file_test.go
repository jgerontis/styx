package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditFilePreviewsThenAppliesExactReplacement(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.txt")
	if err := os.WriteFile(path, []byte("before\nafter\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	tool := NewEditFile(root)
	args := map[string]interface{}{"path": "example.txt", "old_string": "before", "new_string": "changed", "apply": false}

	preview, err := tool.Execute(context.Background(), args)
	if err != nil || !strings.Contains(preview, "- before\n+ changed") {
		t.Fatalf("preview = %q, error = %v", preview, err)
	}
	args["apply"] = true
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("apply edit: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "changed\nafter\n" {
		t.Fatalf("file = %q, error = %v", data, err)
	}
}

func TestEditFileRejectsMissingText(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.txt")
	if err := os.WriteFile(path, []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := NewEditFile(root).Execute(context.Background(), map[string]interface{}{"path": "example.txt", "old_string": "before", "new_string": "again", "apply": true})
	if err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("expected missing text error, got %v", err)
	}
}

func TestEditFileRejectsOutsideWorkspace(t *testing.T) {
	_, err := NewEditFile(t.TempDir()).Execute(context.Background(), map[string]interface{}{"path": "../secret", "old_string": "before", "new_string": "changed", "apply": false})
	if err == nil || !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("expected workspace boundary error, got %v", err)
	}
}

func TestEditFileReplacesWithMultipleLines(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.txt")
	if err := os.WriteFile(path, []byte("first\nsecond\nthird\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	result, err := NewEditFile(root).Execute(context.Background(), map[string]interface{}{"path": "example.txt", "old_string": "second", "new_string": "second\ninserted one\ninserted two", "apply": true})
	if err != nil || !strings.HasPrefix(result, "Applied:") {
		t.Fatalf("result = %q, error = %v", result, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "first\nsecond\ninserted one\ninserted two\nthird\n" {
		t.Fatalf("file = %q, error = %v", data, err)
	}
}

func TestEditFileRejectsAmbiguousText(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.txt")
	if err := os.WriteFile(path, []byte("repeat\nrepeat\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := NewEditFile(root).Execute(context.Background(), map[string]interface{}{"path": "example.txt", "old_string": "repeat", "new_string": "changed", "apply": false})
	if err == nil || !strings.Contains(err.Error(), "appears 2 times") {
		t.Fatalf("expected ambiguous text error, got %v", err)
	}
}
