package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ProviderConfig struct {
	APIKey string `json:"api_key,omitempty"`
	URL    string `json:"url,omitempty"`
	Model  string `json:"model,omitempty"`
}

type Config struct {
	DefaultProvider string                    `json:"default_provider"`
	Providers       map[string]ProviderConfig `json:"providers"`
	AppsDir         string                    `json:"apps_dir"`
	Apps            map[string]string         `json:"apps"`    // AppName -> DirectoryPath
	Prompts         map[string]string         `json:"prompts"` // PromptText -> AppName
	ServerPort      int                       `json:"server_port"`
}

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "promptyly")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func GetConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func LoadConfig() (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		DefaultProvider: "gemini",
		Providers: map[string]ProviderConfig{
			"gemini": {
				Model: "gemini-1.5-flash",
			},
			"claude": {
				Model: "claude-3-5-sonnet-20240620",
			},
			"ollama": {
				URL:   "http://localhost:11434",
				Model: "llama3",
			},
			"lmstudio": {
				URL:   "http://localhost:1234/v1",
				Model: "meta-llama-3-8b-instruct",
			},
		},
		Apps:       make(map[string]string),
		Prompts:    make(map[string]string),
		ServerPort: 6071,
	}

	home, err := os.UserHomeDir()
	if err == nil {
		cfg.AppsDir = filepath.Join(home, "promptyly-apps")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Save default config if it doesn't exist
		if err := SaveConfig(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var loadedConfig Config
	if err := json.Unmarshal(data, &loadedConfig); err != nil {
		return nil, err
	}

	// Merge with defaults to ensure all providers and keys are present
	if loadedConfig.DefaultProvider != "" {
		cfg.DefaultProvider = loadedConfig.DefaultProvider
	}
	if loadedConfig.AppsDir != "" {
		cfg.AppsDir = loadedConfig.AppsDir
	}
	if loadedConfig.Apps != nil {
		cfg.Apps = loadedConfig.Apps
	}
	if loadedConfig.Prompts != nil {
		cfg.Prompts = loadedConfig.Prompts
	}
	if loadedConfig.ServerPort != 0 {
		cfg.ServerPort = loadedConfig.ServerPort
	} else {
		cfg.ServerPort = 6071
	}
	for k, v := range loadedConfig.Providers {
		cfg.Providers[k] = v
	}

	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (cfg *Config) GetActiveProvider() (string, ProviderConfig) {
	prov := cfg.DefaultProvider
	if prov == "" {
		prov = "gemini"
	}
	pc, ok := cfg.Providers[prov]
	if !ok {
		return prov, ProviderConfig{}
	}
	return prov, pc
}
