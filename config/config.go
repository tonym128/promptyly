package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type ProviderConfig struct {
	APIKey string   `json:"api_key,omitempty"`
	URL    string   `json:"url,omitempty"`
	Model  string   `json:"model,omitempty"`
	Models []string `json:"models,omitempty"`
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

func DetectLocalLlamafiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	modelsDir := filepath.Join(home, ".local", "share", "promptyly", "models")
	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		return nil
	}
	var models []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".exe") {
			name = strings.TrimSuffix(name, ".exe")
		}
		if strings.HasSuffix(name, ".llamafile") {
			name = strings.TrimSuffix(name, ".llamafile")
		}
		if strings.HasSuffix(name, ".gguf") {
			name = strings.TrimSuffix(name, ".gguf")
		}
		models = append(models, name)
	}
	return models
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
				Model:  "gemini-1.5-flash",
				Models: []string{"gemini-1.5-flash", "gemini-1.5-pro", "gemini-1.0-pro"},
			},
			"claude": {
				Model:  "claude-3-5-sonnet-20240620",
				Models: []string{"claude-3-5-sonnet-20240620", "claude-3-haiku-20240307", "claude-3-opus-20240229"},
			},
			"ollama": {
				URL:    "http://localhost:11434",
				Model:  "llama3",
				Models: []string{"llama3", "qwen2.5-coder:7b", "qwen2.5-coder:1.5b", "codegemma"},
			},
			"lmstudio": {
				URL:    "http://localhost:1234/v1",
				Model:  "meta-llama-3-8b-instruct",
				Models: []string{"meta-llama-3-8b-instruct", "qwen2.5-coder-1.5b-instruct"},
			},
			"llamafile": {
				URL:    "http://localhost:8080/v1",
				Model:  "qwen2.5-coder-1.5b-instruct",
				Models: []string{"qwen2.5-coder-1.5b-instruct", "llama-3.2-1b-instruct"},
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

	// Detect local llamafiles and add to llamafile models list
	localLlamafiles := DetectLocalLlamafiles()
	if len(localLlamafiles) > 0 {
		lf := cfg.Providers["llamafile"]
		modelSet := make(map[string]bool)
		for _, m := range lf.Models {
			modelSet[m] = true
		}
		for _, m := range localLlamafiles {
			modelSet[m] = true
		}
		var uniqueModels []string
		for m := range modelSet {
			uniqueModels = append(uniqueModels, m)
		}
		lf.Models = uniqueModels
		cfg.Providers["llamafile"] = lf
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
		cfgV := cfg.Providers[k]
		if v.APIKey != "" {
			cfgV.APIKey = v.APIKey
		}
		if v.URL != "" {
			cfgV.URL = v.URL
		}
		if v.Model != "" {
			cfgV.Model = v.Model
		}
		if len(v.Models) > 0 {
			cfgV.Models = v.Models
		}
		cfg.Providers[k] = cfgV
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

type VersionCheckResult struct {
	ServerVersion string `json:"server_version"`
	IsNewer       bool   `json:"is_newer"`
}

func CheckForUpdates(sharingServerURL string) (*VersionCheckResult, error) {
	if sharingServerURL == "" {
		return nil, errors.New("sharing server URL not configured")
	}

	url := fmt.Sprintf("%s/api/version/check?version=%s", strings.TrimSuffix(sharingServerURL, "/"), Version)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	var res VersionCheckResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return &res, nil
}

func GetBinaryName() string {
	osName := runtime.GOOS
	archName := runtime.GOARCH

	// Adjust for android/termux
	if osName == "linux" {
		// Check if running in Termux
		if _, termux := os.LookupEnv("PREFIX"); termux {
			osName = "android"
		}
	}

	ext := ""
	if osName == "windows" {
		ext = ".exe"
	}

	return fmt.Sprintf("promptyly-%s-%s%s", osName, archName, ext)
}

func ApplyUpdate(newBinaryPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	// On non-Windows platforms, we can use the rename-and-replace trick
	if runtime.GOOS != "windows" {
		oldPath := exePath + ".old"
		_ = os.Remove(oldPath) // remove any leftover old path
		
		// Rename current binary to oldPath
		if err := os.Rename(exePath, oldPath); err != nil {
			return err
		}
		
		// Move new binary to exePath
		if err := os.Rename(newBinaryPath, exePath); err != nil {
			// Try to restore old binary if move failed
			_ = os.Rename(oldPath, exePath)
			return err
		}
		
		// Set executable permissions
		_ = os.Chmod(exePath, 0755)
		
		// Clean up old binary
		_ = os.Remove(oldPath)
		return nil
	}

	// On Windows, we need to spawn a background shell process to replace the binary after we exit
	// We'll write a simple powershell command that waits for our process (by PID) to exit,
	// then replaces the binary.
	pid := os.Getpid()
	psCommand := fmt.Sprintf(`Start-Sleep -Seconds 1; while (Get-Process -Id %d -ErrorAction SilentlyContinue) { Start-Sleep -Milliseconds 250 }; Move-Item -Path '%s' -Destination '%s' -Force`, pid, newBinaryPath, exePath)
	
	cmd := exec.Command("powershell", "-Command", psCommand)
	if err := cmd.Start(); err != nil {
		return err
	}
	
	return nil
}

func TriggerSelfUpdate(sharingServerURL string) error {
	if sharingServerURL == "" {
		return errors.New("sharing server URL not configured")
	}

	binaryName := GetBinaryName()
	downloadURL := fmt.Sprintf("%s/binaries/%s", strings.TrimSuffix(sharingServerURL, "/"), binaryName)

	// Download to a temp file in the same directory as the executable to ensure we're on the same volume (crucial for os.Rename)
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exeDir := filepath.Dir(exePath)
	tempFile, err := os.CreateTemp(exeDir, "promptyly-update-*.tmp")
	if err != nil {
		// Fallback to system temp dir if directory is not writeable directly
		tempFile, err = os.CreateTemp("", "promptyly-update-*.tmp")
		if err != nil {
			return err
		}
	}
	tempPath := tempFile.Name()
	defer tempFile.Close()

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_ = os.Remove(tempPath)
		return fmt.Errorf("bad status code from server: %d", resp.StatusCode)
	}

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	tempFile.Close() // Close file handle before renaming/moving

	if err := ApplyUpdate(tempPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	return nil
}
