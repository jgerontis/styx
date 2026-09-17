package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jgerontis/styx/internal/provider"
	"github.com/jgerontis/styx/internal/skill"
)

// LoadSkill loads instructions for a discovered Agent Skill.
type LoadSkill struct {
	registry *skill.Registry
}

// NewLoadSkill creates a Skill activation tool.
func NewLoadSkill(registry *skill.Registry) *LoadSkill {
	return &LoadSkill{registry: registry}
}

// Definition returns the load_skill function schema.
func (t *LoadSkill) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "load_skill",
		Description: "Activate one available Agent Skill and load its instructions. Use only when the task matches the Skill's catalog description.",
		Schema:      json.RawMessage(`{"type":"object","required":["name"],"properties":{"name":{"type":"string","description":"Name of the available skill to activate"}}}`),
	}
}

// Execute loads the selected Skill body as a tool result.
func (t *LoadSkill) Execute(_ context.Context, args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("load_skill requires a skill name")
	}
	selected, ok := t.registry.Get(name)
	if !ok {
		return "", fmt.Errorf("skill %q is not available", name)
	}
	instructions, err := selected.Instructions()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Activated skill: %s\nAllowed tools: %s\n\n%s", selected.Name, strings.Join(selected.AllowedToolNames(), " "), instructions), nil
}

// Skill returns the selected discovered Skill for permission handling.
func (t *LoadSkill) Skill(name string) (skill.Skill, bool) {
	return t.registry.Get(name)
}
