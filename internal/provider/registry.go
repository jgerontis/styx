package provider

import (
	"context"
	"fmt"
	"sync"
)

// Registry manages available providers.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(name string, p Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %q already registered", name)
	}

	r.providers[name] = p
	return nil
}

// Get retrieves a provider by name.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", name)
	}

	return p, nil
}

// All returns all registered providers.
func (r *Registry) All(ctx context.Context) ([]Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}

	return providers, nil
}

// Connect attempts to connect to the specified provider.
func (r *Registry) Connect(ctx context.Context, name string) error {
	p, err := r.Get(name)
	if err != nil {
		return err
	}

	return p.Connect(ctx)
}

// Disconnect attempts to disconnect from the specified provider.
func (r *Registry) Disconnect(ctx context.Context, name string) error {
	p, err := r.Get(name)
	if err != nil {
		return err
	}

	return p.Disconnect(ctx)
}
