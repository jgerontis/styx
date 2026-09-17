package config

// Defaults for configuration.
const (
	DefaultProvider             = "ollama"
	DefaultModel                = "llama2"
	DefaultBaseURL              = "http://localhost:11434"
	DefaultLogLevel             = "info"
	DefaultOutputFormat         = "text"
	DefaultLoopMaxSameToolCalls = 5
	DefaultLoopMaxIterations    = 25
)

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Provider:             DefaultProvider,
		Model:                DefaultModel,
		BaseURL:              DefaultBaseURL,
		LogLevel:             DefaultLogLevel,
		OutputFormat:         DefaultOutputFormat,
		LoopMaxSameToolCalls: DefaultLoopMaxSameToolCalls,
		LoopMaxIterations:    DefaultLoopMaxIterations,
	}
}
