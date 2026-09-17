package tool

import (
	"context"
	"fmt"
	"sync"

	"github.com/jgerontis/styx/internal/provider"
)

// Tool is a model-callable workspace capability.
type Tool interface {
	Definition() provider.ToolDefinition
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

// Registry stores tools by their model-visible name.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Definition().Name
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}
	r.tools[name] = tool
	return nil
}

// Definitions returns all model-visible tool schemas.
func (r *Registry) Definitions() []provider.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	definitions := make([]provider.ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		definitions = append(definitions, tool.Definition())
	}
	return definitions
}

// Execute invokes a registered tool.
func (r *Registry) Execute(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("tool %q is not available", name)
	}
	return tool.Execute(ctx, args)
}
