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

func TestConfigSetPersistsAndConfigShowReflectsIt(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("STYX_CONFIG_FILE", configFile)

	rt := &runtime.Runtime{Config: config.DefaultConfig()}

	set := newConfigSetCommand(rt)
	var setOutput bytes.Buffer
	set.SetOut(&setOutput)
	set.SetArgs([]string{"model", "qwen2.5:7b"})
	if err := set.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("set model: %v", err)
	}
	if !strings.Contains(setOutput.String(), "model = qwen2.5:7b") {
		t.Errorf("set output = %q", setOutput.String())
	}

	data, err := os.ReadFile(configFile)
	if err != nil || !strings.Contains(string(data), "model: qwen2.5:7b") {
		t.Fatalf("persisted config = %q, error = %v", data, err)
	}

	show := newConfigShowCommand(rt)
	var showOutput bytes.Buffer
	show.SetOut(&showOutput)
	if err := show.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("show config: %v", err)
	}
	if !strings.Contains(showOutput.String(), "model: qwen2.5:7b") {
		t.Errorf("show output = %q", showOutput.String())
	}
}

func TestConfigSetRejectsUnknownKey(t *testing.T) {
	t.Setenv("STYX_CONFIG_FILE", filepath.Join(t.TempDir(), "config.yaml"))
	rt := &runtime.Runtime{Config: config.DefaultConfig()}

	set := newConfigSetCommand(rt)
	set.SetArgs([]string{"nonsense", "value"})
	if err := set.ExecuteContext(context.Background()); err == nil || !strings.Contains(err.Error(), "unknown config key") {
		t.Fatalf("expected unknown key error, got %v", err)
	}
}
