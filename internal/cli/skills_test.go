package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgerontis/styx/internal/config"
	"github.com/jgerontis/styx/internal/runtime"
)

func TestSkillsCommandsListMetadataAndShowInstructions(t *testing.T) {
	root := t.TempDir()
	skillPath := filepath.Join(root, "code-review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("---\nname: code-review\ndescription: Reviews code changes.\n---\n\nRead the diff.\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	rt := &runtime.Runtime{Config: &config.Config{SkillsDir: root}}

	list := newSkillsListCommand(rt)
	var listOutput bytes.Buffer
	list.SetOut(&listOutput)
	list.SetArgs(nil)
	if err := list.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("list skills: %v", err)
	}
	if !strings.Contains(listOutput.String(), "code-review\tReviews code changes.\n") {
		t.Errorf("list output = %q", listOutput.String())
	}

	show := newSkillsShowCommand(rt)
	var showOutput bytes.Buffer
	show.SetOut(&showOutput)
	show.SetArgs([]string{"code-review"})
	if err := show.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("show skill: %v", err)
	}
	if showOutput.String() != "\nRead the diff.\n" {
		t.Errorf("show output = %q", showOutput.String())
	}
}
