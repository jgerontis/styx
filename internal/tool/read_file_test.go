package tool

import (
	"context"
	"fmt"
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
	if result != "2: two\n" {
		t.Errorf("result = %q", result)
	}
}

func TestReadFileRejectsOutsideWorkspace(t *testing.T) {
	_, err := NewReadFile(t.TempDir()).Execute(context.Background(), map[string]interface{}{"path": "../secret.txt"})
	if err == nil || !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("expected workspace boundary error, got %v", err)
	}
}

func TestReadFileCapsLineCountAndReportsHowToContinue(t *testing.T) {
	root := t.TempDir()
	var content strings.Builder
	for i := 1; i <= defaultReadFileLineLimit+50; i++ {
		fmt.Fprintf(&content, "line%d\n", i)
	}
	if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	result, err := NewReadFile(root).Execute(context.Background(), map[string]interface{}{"path": "big.txt"})
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if strings.Contains(result, fmt.Sprintf("%d: ", defaultReadFileLineLimit+1)) {
		t.Errorf("expected output capped at %d lines, got %q", defaultReadFileLineLimit, result)
	}
	if !strings.Contains(result, fmt.Sprintf("start_line=%d", defaultReadFileLineLimit+1)) {
		t.Errorf("expected continuation hint, got %q", result)
	}
}

func TestReadFileTruncatesOverlongLines(t *testing.T) {
	root := t.TempDir()
	longLine := strings.Repeat("x", maxReadFileLineLength+100)
	if err := os.WriteFile(filepath.Join(root, "long.txt"), []byte(longLine+"\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	result, err := NewReadFile(root).Execute(context.Background(), map[string]interface{}{"path": "long.txt"})
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(result, "line truncated to") {
		t.Errorf("expected line-truncation marker, got %q", result)
	}
	if strings.Contains(result, strings.Repeat("x", maxReadFileLineLength+1)) {
		t.Errorf("expected line to be capped at %d chars", maxReadFileLineLength)
	}
}
