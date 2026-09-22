package runtime

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jgerontis/styx/internal/config"
	"github.com/jgerontis/styx/internal/event"
	"github.com/jgerontis/styx/internal/log"
	"github.com/jgerontis/styx/internal/provider"
	"github.com/jgerontis/styx/internal/system1"
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
	// System1 is the fast gate consulted before falling back to the full
	// LLM. It is a NoopGate unless config.System1Enabled is set, so Styx
	// runs fully without a System 1 sidecar installed or reachable.
	System1 system1.Gate
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

	// Create event bus
	bus := event.NewBus()
	providers := provider.NewRegistry()
	if err := providers.Register("ollama", provider.NewOllamaProvider(cfg.BaseURL)); err != nil {
		return nil, fmt.Errorf("register Ollama provider: %w", err)
	}

	var gate system1.Gate = system1.NoopGate{}
	if cfg.System1Enabled {
		gate = system1.NewHTTPGate(cfg.System1Endpoint)
	}

	// Create runtime
	rt := &Runtime{
		Config:    cfg,
		Logger:    logger,
		Bus:       bus,
		Providers: providers,
		System1:   gate,
		// Tools, Skills, Approver, Store to be wired up in later phases
	}

	return rt, nil
}
