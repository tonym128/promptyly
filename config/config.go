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
	DefaultProvider  string                    `json:"default_provider"`
	Providers        map[string]ProviderConfig `json:"providers"`
	AppsDir          string                    `json:"apps_dir"`
	Apps             map[string]string         `json:"apps"`    // AppName -> DirectoryPath
	Prompts          map[string]string         `json:"prompts"` // PromptText -> AppName
	SharingServerURL string                    `json:"sharing_server_url"`
	SharingToken     string                    `json:"sharing_token"`
	CheckRemoteFirst bool                      `json:"check_remote_first"`
	ServerPort       int                       `json:"server_port"`
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
		Apps:             make(map[string]string),
		Prompts:          make(map[string]string),
		ServerPort:       6071,
		SharingServerURL: "http://localhost:6072",
		CheckRemoteFirst: true,
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
		// Docker container path translation:
		// If the loaded AppsDir does not exist, but /root/promptyly-apps exists,
		// dynamically resolve it to the container directory.
		if _, err := os.Stat(cfg.AppsDir); os.IsNotExist(err) {
			if _, err := os.Stat("/root/promptyly-apps"); err == nil {
				cfg.AppsDir = "/root/promptyly-apps"
			}
		}
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
	if loadedConfig.SharingServerURL != "" {
		cfg.SharingServerURL = loadedConfig.SharingServerURL
	}
	if loadedConfig.SharingToken != "" {
		cfg.SharingToken = loadedConfig.SharingToken
	}
	cfg.CheckRemoteFirst = loadedConfig.CheckRemoteFirst
	for k, v := range loadedConfig.Providers {
		cfg.Providers[k] = v
	}
	// On startup/load, scan AppsDir for any unregistered/missing app folders and auto-register them
	if syncMissingApps(cfg) {
		_ = SaveConfig(cfg)
	}

	return cfg, nil
}

func syncMissingApps(cfg *Config) bool {
	if cfg.AppsDir == "" {
		return false
	}

	entries, err := os.ReadDir(cfg.AppsDir)
	if err != nil {
		return false // AppsDir might not exist yet, which is fine
	}

	modified := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) > 0 && name[0] == '.' {
			continue
		}

		if _, exists := cfg.Apps[name]; !exists {
			cfg.Apps[name] = filepath.Join(cfg.AppsDir, name)
			modified = true
		}
	}

	return modified
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

// ResolveAppPath returns the verified absolute directory path for a given application.
// If the configured path does not exist, it attempts to resolve it relative to the
// current config's AppsDir or container-specific directories.
func (cfg *Config) ResolveAppPath(appName string) string {
	appPath, ok := cfg.Apps[appName]
	if !ok {
		return ""
	}
	if _, err := os.Stat(appPath); err == nil {
		return appPath
	}
	// Try relative to dynamically translated AppsDir
	base := filepath.Base(appPath)
	if cfg.AppsDir != "" {
		resolvedPath := filepath.Join(cfg.AppsDir, base)
		if _, err := os.Stat(resolvedPath); err == nil {
			return resolvedPath
		}
	}
	// Check under fallback container path
	containerPath := filepath.Join("/root/promptyly-apps", base)
	if _, err := os.Stat(containerPath); err == nil {
		return containerPath
	}
	return appPath
}
