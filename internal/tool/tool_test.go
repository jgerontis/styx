package tool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jgerontis/styx/internal/provider"
)

func TestRegistryValidatesToolArguments(t *testing.T) {
	registry := NewRegistry()
	called := false
	if err := registry.Register(testTool{called: &called}); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	if _, err := registry.Execute(context.Background(), "test", map[string]interface{}{}); err == nil {
		t.Fatal("expected schema validation error")
	}
	if called {
		t.Fatal("tool executed despite invalid arguments")
	}
	if _, err := registry.Execute(context.Background(), "test", map[string]interface{}{"value": "valid"}); err != nil {
		t.Fatalf("execute valid call: %v", err)
	}
	if !called {
		t.Fatal("tool did not execute with valid arguments")
	}
}

type testTool struct{ called *bool }

func (t testTool) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{Name: "test", Schema: json.RawMessage(`{"type":"object","required":["value"],"properties":{"value":{"type":"string"}}}`)}
}

func (t testTool) Execute(_ context.Context, _ map[string]interface{}) (string, error) {
	*t.called = true
	return "ok", nil
}
