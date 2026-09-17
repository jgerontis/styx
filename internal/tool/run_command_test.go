package tool

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestRunCommandUsesWorkspaceAndSeparateArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test fixture uses sh")
	}
	root := t.TempDir()
	result, err := NewRunCommand(root).Execute(context.Background(), map[string]interface{}{"program": "sh", "args": []interface{}{"-c", "printf '%s:%s' \"$PWD\" \"$1\"", "--", "argument with spaces"}})
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if !strings.Contains(result, root+":argument with spaces") {
		t.Errorf("result = %q", result)
	}
}
