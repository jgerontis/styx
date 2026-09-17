package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileReadsRequestedLineRange(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "example.txt"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	result, err := NewReadFile(root).Execute(context.Background(), map[string]interface{}{"path": "example.txt", "start_line": float64(2), "end_line": float64(2)})
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if result != "2:3fc4ccfe | two\n" {
		t.Errorf("result = %q", result)
	}
}

func TestLineHashIsStable(t *testing.T) {
	if got := lineHash("two"); got != "3fc4ccfe" {
		t.Errorf("line hash = %q", got)
	}
}

func TestReadFileRejectsOutsideWorkspace(t *testing.T) {
	_, err := NewReadFile(t.TempDir()).Execute(context.Background(), map[string]interface{}{"path": "../secret.txt"})
	if err == nil || !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("expected workspace boundary error, got %v", err)
	}
}
