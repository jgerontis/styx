package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jgerontis/styx/internal/provider"
)

const commandTimeout = 60 * time.Second
const commandOutputLimit = 64 * 1024

// RunCommand executes an approved command from the workspace root.
type RunCommand struct{ root string }

func NewRunCommand(root string) *RunCommand { return &RunCommand{root: root} }

func (t *RunCommand) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{Name: "run_command", Description: "Run one approved program in the workspace root, normally a focused test or build. Runs without a shell and has timeout and output limits.", Schema: json.RawMessage(`{"type":"object","required":["program"],"properties":{"program":{"type":"string","description":"Executable name, such as go or npm"},"args":{"type":"array","items":{"type":"string"},"description":"Separate command arguments; do not include shell syntax"}}}`)}
}

func (t *RunCommand) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	program, ok := args["program"].(string)
	if !ok || program == "" {
		return "", fmt.Errorf("run_command requires a program")
	}
	commandArgs, err := stringSlice(args["args"])
	if err != nil {
		return "", err
	}
	timedCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	command := exec.CommandContext(timedCtx, program, commandArgs...)
	command.Dir = t.root
	output, err := command.CombinedOutput()
	if len(output) > commandOutputLimit {
		output = append(output[:commandOutputLimit], []byte("\n[output truncated]")...)
	}
	if timedCtx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("command timed out after %s", commandTimeout)
	}
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func stringSlice(value interface{}) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	values, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("run_command args must be strings")
	}
	result := make([]string, len(values))
	for index, value := range values {
		var ok bool
		result[index], ok = value.(string)
		if !ok {
			return nil, fmt.Errorf("run_command args must be strings")
		}
	}
	return result, nil
}
