package runtime

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jgerontis/styx/internal/config"
	"github.com/jgerontis/styx/internal/event"
	"github.com/jgerontis/styx/internal/log"
	"github.com/jgerontis/styx/internal/provider"
)

// Runtime is the dependency injection container for the entire application.
type Runtime struct {
	Config    *config.Config
	Logger    *slog.Logger
	Bus       event.Bus
	Providers *provider.Registry
	Tools     ToolRegistry
	Skills    SkillRegistry
	Approver  Approver
	Store     SessionStore
}

type ToolRegistry interface {
	// GetTool(name string) (tool.Tool, error)
	// RegisterBuiltins() error
	// All(context.Context) ([]tool.Tool, error)
}

type SkillRegistry interface {
	// Discover(context.Context) ([]skill.Skill, error)
	// Get(name string) (*skill.Skill, error)
}

type Approver interface {
	// Check(context.Context, toolCall, skill) (permission.Decision, error)
}

type SessionStore interface {
	// Save(context.Context, *state.Session) error
	// Load(context.Context, id string) (*state.Session, error)
}

// New creates and initializes a new Runtime with all dependencies.
func New(ctx context.Context) (*Runtime, error) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create logger
	logger := log.NewTextLogger(cfg.LogLevel)
	logger.InfoContext(ctx, "runtime initialized", "provider", cfg.Provider, "model", cfg.Model)

	// Create event bus
	bus := event.NewBus()
	providers := provider.NewRegistry()
	if err := providers.Register("ollama", provider.NewOllamaProvider(cfg.BaseURL)); err != nil {
		return nil, fmt.Errorf("register Ollama provider: %w", err)
	}

	// Create runtime
	rt := &Runtime{
		Config:    cfg,
		Logger:    logger,
		Bus:       bus,
		Providers: providers,
		// Tools, Skills, Approver, Store to be wired up in later phases
	}

	return rt, nil
}
