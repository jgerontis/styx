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

func TestEditFileToleratesReadFileLineNumberPrefixes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.txt")
	if err := os.WriteFile(path, []byte("before\nafter\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	// A model that pastes read_file's "N: " output verbatim instead of
	// stripping the prefix should still succeed rather than fail every time.
	args := map[string]interface{}{"path": "example.txt", "old_string": "1: before", "new_string": "changed", "apply": true}
	if _, err := NewEditFile(root).Execute(context.Background(), args); err != nil {
		t.Fatalf("apply edit with line prefix: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "changed\nafter\n" {
		t.Fatalf("file = %q, error = %v", data, err)
	}
}

func TestEditFileDoesNotStripPrefixesThatAreNotLineNumbers(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.txt")
	if err := os.WriteFile(path, []byte("unrelated content\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	args := map[string]interface{}{"path": "example.txt", "old_string": "1: fix this", "new_string": "changed", "apply": true}
	_, err := NewEditFile(root).Execute(context.Background(), args)
	if err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("expected missing text error since the file has no line-numbered text, got %v", err)
	}
}

func TestEditFileToleratesLineWhitespaceDrift(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.go")
	if err := os.WriteFile(path, []byte("func foo() {\n    return 1\n}\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	// A model retyping this block without preserving indentation should still
	// find the unique block instead of failing outright.
	args := map[string]interface{}{"path": "example.go", "old_string": "func foo() {\nreturn 1\n}", "new_string": "func foo() {\n    return 2\n}", "apply": true}
	if _, err := NewEditFile(root).Execute(context.Background(), args); err != nil {
		t.Fatalf("apply edit with whitespace drift: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "func foo() {\n    return 2\n}\n" {
		t.Fatalf("file = %q, error = %v", data, err)
	}
}

func TestEditFileRefusesAmbiguousWhitespaceDrift(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.go")
	// Two blocks whose lines are identical once trimmed, but with different
	// raw indentation, so the whitespace-tolerant fallback must refuse rather
	// than silently pick one.
	content := "  if true {\n    return 1\n  }\n\nif true {\n    return 1\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	args := map[string]interface{}{"path": "example.go", "old_string": "if true {\nreturn 1\n}", "new_string": "changed", "apply": true}
	_, err := NewEditFile(root).Execute(context.Background(), args)
	if err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("expected ambiguous drift to be refused, got %v", err)
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
