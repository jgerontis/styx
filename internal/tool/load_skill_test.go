package tool

import (
	"context"
	"strings"
	"testing"

	"github.com/jgerontis/styx/internal/skill"
)

func TestLoadSkillReturnsInstructionsAndAllowedTools(t *testing.T) {
	registry := skill.NewRegistryWithBuiltins(skill.Builtins())
	if err := registry.Discover(); err != nil {
		t.Fatalf("discover skills: %v", err)
	}
	result, err := NewLoadSkill(registry).Execute(context.Background(), map[string]interface{}{"name": "ast-grep"})
	if err != nil {
		t.Fatalf("load skill: %v", err)
	}
	for _, expected := range []string{"Activated skill: ast-grep", "Allowed tools: search_structure search_text read_file", "# ast-grep Structural Search"} {
		if !strings.Contains(result, expected) {
			t.Errorf("expected %q in %q", expected, result)
		}
	}
}
