package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchStructureInvokesAstGrepWithStructuredArguments(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "ast-grep")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0o755); err != nil {
		t.Fatalf("write ast-grep fixture: %v", err)
	}

	tool := &SearchStructure{root: root, binary: binary}
	result, err := tool.Execute(context.Background(), map[string]interface{}{"pattern": "func $NAME() {}", "language": "go", "path": "."})
	if err != nil {
		t.Fatalf("search code: %v", err)
	}
	for _, expected := range []string{"--pattern", "func $NAME() {}", "--lang", "go", root} {
		if !strings.Contains(result, expected) {
			t.Errorf("expected %q in %q", expected, result)
		}
	}
}

func TestSearchStructureReportsMissingAstGrep(t *testing.T) {
	tool := &SearchStructure{root: t.TempDir(), binary: "missing-ast-grep"}
	_, err := tool.Execute(context.Background(), map[string]interface{}{"pattern": "func $NAME() {}", "language": "go"})
	if err == nil || !strings.Contains(err.Error(), "requires ast-grep") {
		t.Fatalf("expected installation guidance, got %v", err)
	}
}

func TestSearchStructureUsesConfiguredBinary(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "configured-ast-grep")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf 'configured'\n"), 0o755); err != nil {
		t.Fatalf("write ast-grep fixture: %v", err)
	}
	t.Setenv("STYX_AST_GREP_PATH", binary)

	result, err := NewSearchStructure(root).Execute(context.Background(), map[string]interface{}{"pattern": "func $NAME() {}", "language": "go"})
	if err != nil {
		t.Fatalf("search code: %v", err)
	}
	if result != "configured" {
		t.Errorf("result = %q", result)
	}
}

func TestSearchStructureRequiresLanguage(t *testing.T) {
	_, err := NewSearchStructure(t.TempDir()).Execute(context.Background(), map[string]interface{}{"pattern": "func $NAME() {}"})
	if err == nil || !strings.Contains(err.Error(), "requires a language") {
		t.Fatalf("expected language error, got %v", err)
	}
}
