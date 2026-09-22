package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgerontis/styx/internal/skill"
)

func TestLoadSkillReturnsInstructionsAndAllowedTools(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "code-review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create skill directory: %v", err)
	}
	content := "---\nname: code-review\ndescription: Reviews code changes and identifies defects.\nallowed-tools: read_file search_text\n---\n\n# Code Review\n\nInspect relevant files first.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	registry := skill.NewRegistry(root)
	if err := registry.Discover(); err != nil {
		t.Fatalf("discover skills: %v", err)
	}
	result, err := NewLoadSkill(registry).Execute(context.Background(), map[string]interface{}{"name": "code-review"})
	if err != nil {
		t.Fatalf("load skill: %v", err)
	}
	for _, expected := range []string{"Activated skill: code-review", "Allowed tools: read_file search_text", "# Code Review"} {
		if !strings.Contains(result, expected) {
			t.Errorf("expected %q in %q", expected, result)
		}
	}
}

func TestLoadSkillRequiresAName(t *testing.T) {
	registry := skill.NewRegistry(t.TempDir())
	if err := registry.Discover(); err != nil {
		t.Fatalf("discover skills: %v", err)
	}
	_, err := NewLoadSkill(registry).Execute(context.Background(), map[string]interface{}{})
	if err == nil || !strings.Contains(err.Error(), "requires a skill name") {
		t.Fatalf("expected missing-name error, got %v", err)
	}
}

func TestLoadSkillRejectsUnknownSkill(t *testing.T) {
	registry := skill.NewRegistry(t.TempDir())
	if err := registry.Discover(); err != nil {
		t.Fatalf("discover skills: %v", err)
	}
	_, err := NewLoadSkill(registry).Execute(context.Background(), map[string]interface{}{"name": "missing"})
	if err == nil || !strings.Contains(err.Error(), `skill "missing" is not available`) {
		t.Fatalf("expected not-available error, got %v", err)
	}
}
