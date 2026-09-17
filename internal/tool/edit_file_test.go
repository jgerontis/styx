package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditFilePreviewsThenAppliesAnchoredReplacement(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.txt")
	if err := os.WriteFile(path, []byte("before\nafter\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	tool := NewEditFile(root)
	args := map[string]interface{}{"path": "example.txt", "operations": []interface{}{map[string]interface{}{"kind": "replace", "anchor": "1:" + lineHash("before"), "content": "changed"}}, "apply": false}

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

func TestEditFileRejectsStaleAnchor(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.txt")
	if err := os.WriteFile(path, []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := NewEditFile(root).Execute(context.Background(), map[string]interface{}{"path": "example.txt", "operations": []interface{}{map[string]interface{}{"kind": "replace", "anchor": "1:" + lineHash("before"), "content": "again"}}, "apply": true})
	if err == nil || !strings.Contains(err.Error(), "stale anchor") {
		t.Fatalf("expected stale anchor error, got %v", err)
	}
}

func TestEditFileRejectsOutsideWorkspace(t *testing.T) {
	_, err := NewEditFile(t.TempDir()).Execute(context.Background(), map[string]interface{}{"path": "../secret", "operations": []interface{}{map[string]interface{}{"kind": "replace", "anchor": "1:abcd", "content": "changed"}}, "apply": false})
	if err == nil || !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("expected workspace boundary error, got %v", err)
	}
}

func TestEditFileAppliesMultipleOperationsAtomically(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.txt")
	if err := os.WriteFile(path, []byte("first\nsecond\nthird\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	operations := []interface{}{
		map[string]interface{}{"kind": "replace", "anchor": "1:" + lineHash("first"), "content": "updated"},
		map[string]interface{}{"kind": "insert_after", "anchor": "2:" + lineHash("second"), "content": "inserted one\ninserted two"},
		map[string]interface{}{"kind": "delete", "anchor": "3:" + lineHash("third")},
	}
	result, err := NewEditFile(root).Execute(context.Background(), map[string]interface{}{"path": "example.txt", "operations": operations, "apply": true})
	if err != nil || !strings.HasPrefix(result, "Applied:") {
		t.Fatalf("result = %q, error = %v", result, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "updated\nsecond\ninserted one\ninserted two\n" {
		t.Fatalf("file = %q, error = %v", data, err)
	}
}
