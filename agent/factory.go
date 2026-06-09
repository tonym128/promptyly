package agent

import (
	"fmt"
	"promptyly/config"
)

// NewClient returns the appropriate Agent interface implementation based on provider and config.
func NewClient(provider string, cfg config.ProviderConfig) (Agent, error) {
	switch provider {
	case "gemini":
		return NewGeminiClient(cfg), nil
	case "claude":
		return NewClaudeClient(cfg), nil
	case "ollama", "lmstudio", "openai", "custom":
		return NewOpenAIClient(cfg, provider), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", provider)
	}
}
