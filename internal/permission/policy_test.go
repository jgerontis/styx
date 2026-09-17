package permission

import "testing"

func TestPolicySeparatesReadOnlyAndMutatingTools(t *testing.T) {
	policy := Policy{}
	for _, toolName := range []string{"list_files", "read_file", "search_text", "search_structure"} {
		if policy.Check(toolName) != Allow {
			t.Errorf("expected %s to be allowed", toolName)
		}
	}
	for _, toolName := range []string{"edit_file", "write_file", "run_command", "unknown_tool"} {
		if policy.Check(toolName) != Prompt {
			t.Errorf("expected %s to require approval", toolName)
		}
	}
}

func TestPolicyAllowsToolsGrantedByAnActivatedSkill(t *testing.T) {
	policy := Policy{AllowedTools: []string{"run_command"}}
	if policy.Check("run_command") != Allow {
		t.Error("expected activated Skill to pre-approve run_command")
	}
}
