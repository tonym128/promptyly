package agent

import (
	"fmt"
	"promptyly/config"
)

func NewClient(provider string, cfg config.ProviderConfig) (Agent, error) {
	engineType := cfg.Type
	if engineType == "" {
		engineType = provider
	}
	switch engineType {
	case "gemini":
		return NewGeminiClient(cfg), nil
	case "claude":
		return NewClaudeClient(cfg), nil
	case "ollama", "lmstudio", "openai", "custom", "openai-compatible", "llamafile", "registry":
		return NewOpenAIClient(cfg, engineType), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider type: %s", engineType)
	}
}
