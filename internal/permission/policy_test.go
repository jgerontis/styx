package permission

import (
	"context"
	"errors"
	"testing"

	"github.com/jgerontis/styx/internal/system1"
)

type stubGate struct {
	answer system1.Answer
	err    error
}

func (s stubGate) Answer(context.Context, system1.Question, map[string]any) (system1.Answer, error) {
	return s.answer, s.err
}

func TestEvaluateNeverAutoAllowsWithoutAutoApprove(t *testing.T) {
	policy := Policy{Gate: stubGate{answer: system1.Answer{Noul: 0.99, Confidence: 0.99}}}
	if policy.Evaluate(context.Background(), "run_command", nil) != Prompt {
		t.Error("expected Prompt when AutoApprove is not set, regardless of Gate confidence")
	}
}

func TestEvaluateAutoAllowsOnHighConfidenceSafe(t *testing.T) {
	policy := Policy{
		AutoApprove: true,
		Gate:        stubGate{answer: system1.Answer{Noul: 0.95, Confidence: 0.95}},
	}
	if policy.Evaluate(context.Background(), "run_command", nil) != Allow {
		t.Error("expected Allow on high-confidence safe answer with AutoApprove set")
	}
}

func TestEvaluateFailsSafeOnLowConfidenceOrError(t *testing.T) {
	lowConfidence := Policy{
		AutoApprove: true,
		Gate:        stubGate{answer: system1.Answer{Noul: 0.95, Confidence: 0.5}},
	}
	if lowConfidence.Evaluate(context.Background(), "run_command", nil) != Prompt {
		t.Error("expected Prompt on low-confidence Gate answer")
	}

	gateError := Policy{
		AutoApprove: true,
		Gate:        stubGate{err: errors.New("sidecar unreachable")},
	}
	if gateError.Evaluate(context.Background(), "run_command", nil) != Prompt {
		t.Error("expected Prompt when Gate returns an error")
	}
}

func TestPolicySeparatesReadOnlyAndMutatingTools(t *testing.T) {
	policy := Policy{}
	for _, toolName := range []string{"list_files", "read_file", "search_text"} {
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
