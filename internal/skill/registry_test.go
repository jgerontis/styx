package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryDiscoversMetadataAndLazilyLoadsInstructions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "code-review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create skill directory: %v", err)
	}
	content := "---\nname: code-review\ndescription: Reviews code changes and identifies defects.\nallowed-tools: read_file search_text\n---\n\n# Code Review\n\nInspect relevant files first.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	registry := NewRegistry(root)
	if err := registry.Discover(); err != nil {
		t.Fatalf("discover skills: %v", err)
	}
	skills := registry.All()
	if len(skills) != 1 || skills[0].Name != "code-review" || skills[0].AllowedTools != "read_file search_text" {
		t.Fatalf("skills = %#v", skills)
	}
	instructions, err := skills[0].Instructions()
	if err != nil || instructions != "\n# Code Review\n\nInspect relevant files first.\n" {
		t.Fatalf("instructions = %q, error = %v", instructions, err)
	}
}

func TestRegistryRejectsInvalidSkillName(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "wrong-directory", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create skill directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("---\nname: Wrong_Name\ndescription: Invalid name.\n---\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	if err := NewRegistry(root).Discover(); err == nil {
		t.Fatal("expected invalid skill error")
	}
}

func TestRegistryUsesLaterRootsToOverrideEarlierSkills(t *testing.T) {
	global := t.TempDir()
	local := t.TempDir()
	writeSkill := func(root, description string) {
		path := filepath.Join(root, "code-review", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create skill directory: %v", err)
		}
		if err := os.WriteFile(path, []byte("---\nname: code-review\ndescription: "+description+"\n---\n"), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}
	writeSkill(global, "Global instructions.")
	writeSkill(local, "Local instructions.")

	registry := NewRegistry(global, local)
	if err := registry.Discover(); err != nil {
		t.Fatalf("discover skills: %v", err)
	}
	available, ok := registry.Get("code-review")
	if !ok || available.Description != "Local instructions." {
		t.Fatalf("skill = %#v", available)
	}
}
