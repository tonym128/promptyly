package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"promptyly/app"
	"promptyly/config"
	"promptyly/server"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Register API callbacks for the background daemon
	server.CreateAppCallback = func(prompt string, onToken func(token string)) (string, string, error) {
		freshCfg, err := config.LoadConfig()
		if err != nil {
			return "", "", fmt.Errorf("failed to reload config: %v", err)
		}
		return app.CreateApp(freshCfg, prompt, onToken)
	}
	server.EditAppCallback = func(name, prompt string, onToken func(token string)) error {
		freshCfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to reload config: %v", err)
		}
		return app.EditApp(freshCfg, name, prompt, onToken)
	}
	server.RenameAppCallback = func(oldName, newName string) (string, error) {
		freshCfg, err := config.LoadConfig()
		if err != nil {
			return "", fmt.Errorf("failed to reload config: %v", err)
		}
		return app.RenameApp(freshCfg, oldName, newName)
	}
	server.LinkAppCallback = func(path string) (string, error) {
		freshCfg, err := config.LoadConfig()
		if err != nil {
			return "", fmt.Errorf("failed to reload config: %v", err)
		}
		return app.LinkApp(freshCfg, path)
	}
	server.UnlinkAppCallback = func(name string) error {
		freshCfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to reload config: %v", err)
		}
		return app.UnlinkApp(freshCfg, name)
	}
	server.DeleteAppCallback = func(name string, deleteFolder bool) error {
		freshCfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to reload config: %v", err)
		}
		if deleteFolder {
			return app.DeleteApp(freshCfg, name)
		}
		return app.UnlinkApp(freshCfg, name)
	}
	server.ExportAppCallback = func(name, zipPath string) error {
		freshCfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to reload config: %v", err)
		}
		return app.ExportApp(freshCfg, name, zipPath)
	}
	server.ImportAppCallback = func(zipPath string) (string, error) {
		freshCfg, err := config.LoadConfig()
		if err != nil {
			return "", fmt.Errorf("failed to reload config: %v", err)
		}
		return app.ImportApp(freshCfg, zipPath)
	}
	server.UpdateMetadataCallback = func(name, newName, newPrompt string) (string, error) {
		freshCfg, err := config.LoadConfig()
		if err != nil {
			return "", fmt.Errorf("failed to reload config: %v", err)
		}
		currentName := name
		var err2 error
		if newName != "" && newName != name {
			currentName, err2 = app.RenameApp(freshCfg, name, newName)
			if err2 != nil {
				return "", err2
			}
		}

		oldPrompt := ""
		for pr, appName := range freshCfg.Prompts {
			if appName == currentName {
				oldPrompt = pr
				break
			}
		}

		if newPrompt != oldPrompt {
			if oldPrompt != "" {
				delete(freshCfg.Prompts, oldPrompt)
			}
			if newPrompt != "" {
				freshCfg.Prompts[newPrompt] = currentName
			}
			if err := config.SaveConfig(freshCfg); err != nil {
				return "", err
			}
		}

		return currentName, nil
	}


	if len(os.Args) < 2 {
		printHelp()
		return
	}

	port := cfg.ServerPort
	if port == 0 {
		port = 6071
	}

	command := strings.ToLower(os.Args[1])

	switch command {
	case "version":
		fmt.Printf("Promptyly version %s\n", config.Version)
		return

	case "help":
		printHelp()

	case "config":
		if len(os.Args) < 3 {
			fmt.Println("Usage: promptyly config [setup | show | set <key> <value>]")
			return
		}
		subcmd := strings.ToLower(os.Args[2])
		switch subcmd {
		case "setup":
			handleConfigSetup(cfg)
		case "show":
			handleConfigShow(cfg)
		case "set":
			if len(os.Args) < 5 {
				fmt.Println("Usage: promptyly config set <key> <value>")
				return
			}
			handleConfigSet(cfg, os.Args[3], os.Args[4])
		default:
			fmt.Printf("Unknown config subcommand: %s\n", subcmd)
		}

	case "create":
		if len(os.Args) < 3 {
			fmt.Println("Usage: promptyly create \"<prompt>\"")
			return
		}
		promptVal := os.Args[2]
		fmt.Println("Generating application via Promptyly server...")
		reqBody, _ := json.Marshal(map[string]string{"prompt": promptVal})
		appName, _, err := streamCreateRequest(port, reqBody)
		if err != nil {
			fmt.Printf("❌ Creation failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Generated application: %s\n", appName)
		err = app.InteractiveSession(cfg, appName)
		if err != nil {
			fmt.Printf("❌ Run session failed: %v\n", err)
			os.Exit(1)
		}

	case "run":
		if len(os.Args) < 3 {
			fmt.Println("Usage: promptyly run <app-name>")
			return
		}
		appName := os.Args[2]
		err = ensureServerRunning(port)
		if err != nil {
			fmt.Printf("❌ Failed to start background server: %v\n", err)
			os.Exit(1)
		}
		err = app.InteractiveSession(cfg, appName)
		if err != nil {
			fmt.Printf("❌ Run session failed: %v\n", err)
			os.Exit(1)
		}

	case "list":
		respBytes, err := sendServerRequest(port, "GET", "/api/apps", nil)
		if err != nil {
			fmt.Printf("❌ List failed: %v\n", err)
			os.Exit(1)
		}

		type AppInfo struct {
			Path   string `json:"path"`
			Prompt string `json:"prompt"`
		}
		var apps map[string]AppInfo
		if err := json.Unmarshal(respBytes, &apps); err != nil {
			fmt.Printf("❌ Failed to parse response: %v\n", err)
			os.Exit(1)
		}

		if len(apps) == 0 {
			fmt.Println("No applications created yet. Build one with `promptyly create \"<prompt>\"`!")
			return
		}

		fmt.Println("Saved Applications:")
		var names []string
		for name := range apps {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			info := apps[name]
			fmt.Printf("  - %-30s -> Path: %s\n", name, info.Path)
		}

	case "export":
		if len(os.Args) < 4 {
			fmt.Println("Usage: promptyly export <app-name> <output.zip>")
			return
		}
		appName := os.Args[2]
		zipPath := os.Args[3]
		absZipPath, err := filepath.Abs(zipPath)
		if err != nil {
			absZipPath = zipPath
		}

		fmt.Printf("Exporting application '%s' to %s via Promptyly server...\n", appName, absZipPath)
		reqBody, _ := json.Marshal(map[string]string{
			"name":    appName,
			"zipPath": absZipPath,
		})
		_, err = sendServerRequest(port, "POST", "/api/apps/export", reqBody)
		if err != nil {
			fmt.Printf("❌ Export failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Export successful!")

	case "import":
		if len(os.Args) < 3 {
			fmt.Println("Usage: promptyly import <zip-path>")
			return
		}
		zipPath := os.Args[2]
		absZipPath, err := filepath.Abs(zipPath)
		if err != nil {
			absZipPath = zipPath
		}

		fmt.Printf("Importing application from %s via Promptyly server...\n", absZipPath)
		reqBody, _ := json.Marshal(map[string]string{
			"zipPath": absZipPath,
		})
		respBytes, err := sendServerRequest(port, "POST", "/api/apps/import", reqBody)
		if err != nil {
			fmt.Printf("❌ Import failed: %v\n", err)
			os.Exit(1)
		}

		var respData struct {
			Success bool   `json:"success"`
			AppName string `json:"appName"`
		}
		if err := json.Unmarshal(respBytes, &respData); err != nil {
			fmt.Printf("❌ Failed to parse response: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ App '%s' successfully imported and registered!\n", respData.AppName)

		fmt.Printf("Would you like to run '%s' now? (y/n) [y]: ", respData.AppName)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
			if ans == "" || ans == "y" || ans == "yes" {
				err = app.InteractiveSession(cfg, respData.AppName)
				if err != nil {
					fmt.Printf("❌ Run session failed: %v\n", err)
					os.Exit(1)
				}
			}
		}

	case "serve", "daemon":
		_, err := server.StartDevServer(port)
		if err != nil {
			fmt.Printf("❌ Failed to start server: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("🚀 Promptyly background server/API running on http://127.0.0.1:%d\n", port)
		fmt.Println("Press Ctrl+C to exit.")
		select {}

	case "publish":
		var appName string
		if len(os.Args) < 3 {
			var err error
			appName, err = selectAppInteractively(cfg, port)
			if err != nil {
				fmt.Printf("❌ Failed: %v\n", err)
				os.Exit(1)
			}
			if appName == "" {
				fmt.Println("Cancelled.")
				return
			}
		} else {
			appName = os.Args[2]
		}
		description := promptInput("Enter an optional description: ")

		fmt.Printf("Publishing application '%s' via Promptyly server...\n", appName)
		reqBody, _ := json.Marshal(map[string]string{
			"name":        appName,
			"description": description,
		})
		respBytes, err := sendServerRequest(port, "POST", "/api/apps/publish", reqBody)
		if err != nil {
			fmt.Printf("❌ Publish failed: %v\n", err)
			os.Exit(1)
		}

		var respData struct {
			Success   bool   `json:"success"`
			AppID     string `json:"appId"`
			LiveURL   string `json:"liveUrl"`
			DetailURL string `json:"detailUrl"`
		}
		if err := json.Unmarshal(respBytes, &respData); err != nil {
			fmt.Printf("❌ Failed to parse response: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n==================================================\n")
		fmt.Printf("✅ Application successfully published to the registry!\n")
		fmt.Printf("👉 Live URL: %s\n", respData.LiveURL)
		fmt.Printf("👉 Detail Page: %s\n", respData.DetailURL)
		fmt.Printf("==================================================\n\n")

	case "search":
		if len(os.Args) < 3 {
			fmt.Println("Usage: promptyly search \"<query>\"")
			return
		}
		query := os.Args[2]
		fmt.Printf("Searching registry via Promptyly server for '%s'...\n", query)

		path := fmt.Sprintf("/api/apps/search?q=%s", url.QueryEscape(query))
		respBytes, err := sendServerRequest(port, "GET", path, nil)
		if err != nil {
			fmt.Printf("❌ Search failed: %v\n", err)
			os.Exit(1)
		}

		type RemoteApp struct {
			ID          string    `json:"id"`
			Username    string    `json:"username"`
			Name        string    `json:"name"`
			Prompt      string    `json:"prompt"`
			Description string    `json:"description"`
			Views       int       `json:"views"`
			Downloads   int       `json:"downloads"`
			CreatedAt   time.Time `json:"created_at"`
		}

		var list []RemoteApp
		if err := json.Unmarshal(respBytes, &list); err != nil {
			fmt.Printf("❌ Failed to parse response: %v\n", err)
			os.Exit(1)
		}

		if len(list) == 0 {
			fmt.Println("No matching applications found on the sharing server.")
			return
		}

		fmt.Println("Search Results:")
		fmt.Println("--------------------------------------------------------------------------------")
		for _, app := range list {
			fmt.Printf("ID:          %s\n", app.ID)
			fmt.Printf("Name:        %s\n", app.Name)
			fmt.Printf("Prompt:      \"%s\"\n", app.Prompt)
			if app.Description != "" {
				fmt.Printf("Description: %s\n", app.Description)
			}
			fmt.Printf("Stats:       by %s | %d views | %d downloads | Created %s\n",
				app.Username, app.Views, app.Downloads, app.CreatedAt.Format("2006-01-02"))
			fmt.Println("--------------------------------------------------------------------------------")
		}

	case "download":
		if len(os.Args) < 3 {
			fmt.Println("Usage: promptyly download <app-id>")
			return
		}
		appID := os.Args[2]
		fmt.Printf("Downloading application '%s' via Promptyly server...\n", appID)
		reqBody, _ := json.Marshal(map[string]string{
			"appId": appID,
		})
		respBytes, err := sendServerRequest(port, "POST", "/api/apps/download", reqBody)
		if err != nil {
			fmt.Printf("❌ Download failed: %v\n", err)
			os.Exit(1)
		}

		var respData struct {
			Success bool   `json:"success"`
			AppName string `json:"appName"`
		}
		if err := json.Unmarshal(respBytes, &respData); err != nil {
			fmt.Printf("❌ Failed to parse response: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n✅ App '%s' successfully downloaded and registered!\n", respData.AppName)
		fmt.Printf("👉 Run it locally: promptyly run %s\n\n", respData.AppName)

	case "register":
		fmt.Println("Registering custom protocol URL scheme via Promptyly server...")
		_, err := sendServerRequest(port, "POST", "/api/protocol/register", nil)
		if err != nil {
			fmt.Printf("❌ Registration failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Successfully registered prompt:// URL scheme handler.")

	case "unregister":
		fmt.Println("Unregistering custom protocol URL scheme via Promptyly server...")
		_, err := sendServerRequest(port, "POST", "/api/protocol/unregister", nil)
		if err != nil {
			fmt.Printf("❌ Unregistration failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Successfully unregistered prompt:// URL scheme handler.")

	case "handle":
		if len(os.Args) < 3 {
			fmt.Println("Usage: promptyly handle \"<prompt-url>\"")
			return
		}
		handleURL(cfg, os.Args[2])

	default:
		// Attempt to parse as a URL handler in case of direct OS callback
		if strings.HasPrefix(command, "prompt://") {
			handleURL(cfg, os.Args[1])
		} else {
			fmt.Printf("Unknown command: %s\n", command)
			printHelp()
		}
	}
}

func printHelp() {
	helpText := `
PROMPTYLY - Own the prompt:// url, instantly build websites, and edit them in real-time.

Usage:
  promptyly <command> [arguments]

Commands:
  version                 Show the version of Promptyly.
  create "<prompt>"       Generates a new app, starts the server and begins interactive editing.
  run <app-name>          Runs the local dev server and starts the interactive editing terminal.
  serve                   Starts the background dev server and REST API daemon.
  list                    Lists all locally generated applications.
  export <app-name> <zip> Packages the application into a zip file for sharing.
  import <zip-path>       Imports a zipped application and registers it locally.
  publish <app-name>      Uploads the app to the remote sharing registry server.
  search "<query>"        Queries the remote sharing server for matching web apps.
  download <app-id>       Downloads and imports an app from the sharing server.
  register                Registers the prompt:// URL scheme for browser-level deep links.
  unregister              Unregisters the prompt:// URL scheme from the operating system.
  config setup            Interactive setup guide for API Keys and LLM providers.
  config set <key> <val>  Manually set configuration settings.
  config show             Show current configuration details.
  help                    Show this help message.

Examples:
  promptyly config setup
  promptyly create "A sleek dark mode pomodoro timer with custom audio loops and task cards"
  promptyly run a-sleek-dark-mode-pomodoro-timer
  promptyly export a-sleek-dark-mode-pomodoro-timer my-pomodoro.zip
  promptyly import my-pomodoro.zip
  promptyly handle "prompt://create?prompt=Simple+Calculator"
`
	fmt.Println(helpText)
}

func handleConfigShow(cfg *config.Config) {
	fmt.Printf("Configuration Path: %s\n", filepath.Join(os.Getenv("HOME"), ".config", "promptyly", "config.json"))
	defaultProviderName := cfg.DefaultProvider
	if defaultProviderName == "lmstudio" {
		defaultProviderName = "openai-compatible"
	}
	fmt.Printf("Default LLM Provider: %s\n", defaultProviderName)
	fmt.Printf("Apps Output Directory: %s\n", cfg.AppsDir)
	fmt.Printf("Server Port: %d\n", cfg.ServerPort)
	fmt.Println("\nProviders Configured:")
	for k, v := range cfg.Providers {
		status := "Configured"
		if v.APIKey == "" && (k == "gemini" || k == "claude") {
			status = "Missing Key"
		}
		urlStr := ""
		if v.URL != "" {
			urlStr = fmt.Sprintf(" (URL: %s)", v.URL)
		}
		displayName := k
		if k == "lmstudio" {
			displayName = "openai-compatible"
		}
		fmt.Printf("  - %-18s -> Model: %-30s | Status: %s%s\n", displayName, v.Model, status, urlStr)
	}
}

func handleConfigSet(cfg *config.Config, key, val string) {
	switch strings.ToLower(key) {
	case "default_provider":
		valLower := strings.ToLower(val)
		if valLower == "openai-compatible" || valLower == "openai_compatible" || valLower == "openai" {
			cfg.DefaultProvider = "lmstudio"
		} else {
			cfg.DefaultProvider = valLower
		}
	case "gemini_key":
		p := cfg.Providers["gemini"]
		p.APIKey = val
		cfg.Providers["gemini"] = p
	case "gemini_model":
		p := cfg.Providers["gemini"]
		p.Model = val
		cfg.Providers["gemini"] = p
	case "claude_key":
		p := cfg.Providers["claude"]
		p.APIKey = val
		cfg.Providers["claude"] = p
	case "claude_model":
		p := cfg.Providers["claude"]
		p.Model = val
		cfg.Providers["claude"] = p
	case "ollama_url":
		p := cfg.Providers["ollama"]
		p.URL = val
		cfg.Providers["ollama"] = p
	case "ollama_model":
		p := cfg.Providers["ollama"]
		p.Model = val
		cfg.Providers["ollama"] = p
	case "lmstudio_url", "openai_url", "openai_compatible_url":
		p := cfg.Providers["lmstudio"]
		p.URL = val
		cfg.Providers["lmstudio"] = p
	case "lmstudio_model", "openai_model", "openai_compatible_model":
		p := cfg.Providers["lmstudio"]
		p.Model = val
		cfg.Providers["lmstudio"] = p
	case "lmstudio_key", "openai_key", "openai_compatible_key":
		p := cfg.Providers["lmstudio"]
		p.APIKey = val
		cfg.Providers["lmstudio"] = p
	case "apps_dir":
		cfg.AppsDir = val
	case "server_port":
		port, err := strconv.Atoi(val)
		if err != nil {
			fmt.Printf("❌ Invalid port value: %s (must be a number)\n", val)
			return
		}
		cfg.ServerPort = port
	case "sharing_server_url", "sharing_url":
		cfg.SharingServerURL = val
	case "sharing_token":
		cfg.SharingToken = val
	case "check_remote_first":
		cfg.CheckRemoteFirst = (strings.ToLower(val) == "true" || val == "1" || strings.ToLower(val) == "yes")
	default:
		fmt.Printf("❌ Unknown configuration key: %s\n", key)
		return
	}

	if err := config.SaveConfig(cfg); err != nil {
		fmt.Printf("❌ Failed to save configuration: %v\n", err)
	} else {
		fmt.Printf("✅ Set: %s = %s\n", key, val)
	}
}

func handleConfigSetup(cfg *config.Config) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("\n--- Promptyly Setup Guide ---")
	fmt.Println("Configure credentials for LLM providers (Gemini, Claude, Ollama, OpenAI-compatible).")
	fmt.Println("--------------------------------------------------")

	// 1. Choose Provider
	fmt.Println("Select default LLM provider:")
	fmt.Println("1) Gemini (Recommended - Google)")
	fmt.Println("2) Claude (Anthropic)")
	fmt.Println("3) Ollama (Local LLM Server)")
	fmt.Println("4) OpenAI-compatible (LM Studio, Local AI, etc.)")
	fmt.Print("Choose option (1-4) [default: 1]: ")

	provider := "gemini"
	if scanner.Scan() {
		choice := strings.TrimSpace(scanner.Text())
		switch choice {
		case "2":
			provider = "claude"
		case "3":
			provider = "ollama"
		case "4":
			provider = "lmstudio"
		}
	}
	cfg.DefaultProvider = provider
	fmt.Printf("Selected default provider: %s\n\n", provider)

	pCfg := cfg.Providers[provider]

	switch provider {
	case "gemini":
		fmt.Print("Enter Gemini API Key: ")
		if scanner.Scan() {
			val := strings.TrimSpace(scanner.Text())
			if val != "" {
				pCfg.APIKey = val
			}
		}
		fmt.Print("Enter Gemini Model [default: gemini-1.5-flash]: ")
		if scanner.Scan() {
			val := strings.TrimSpace(scanner.Text())
			if val != "" {
				pCfg.Model = val
			}
		}

	case "claude":
		fmt.Print("Enter Claude API Key: ")
		if scanner.Scan() {
			val := strings.TrimSpace(scanner.Text())
			if val != "" {
				pCfg.APIKey = val
			}
		}
		fmt.Print("Enter Claude Model [default: claude-3-5-sonnet-20240620]: ")
		if scanner.Scan() {
			val := strings.TrimSpace(scanner.Text())
			if val != "" {
				pCfg.Model = val
			}
		}

	case "ollama":
		fmt.Print("Enter Ollama Endpoint URL [default: http://localhost:11434]: ")
		if scanner.Scan() {
			val := strings.TrimSpace(scanner.Text())
			if val != "" {
				pCfg.URL = val
			}
		}
		fmt.Print("Enter Ollama Model [default: llama3]: ")
		if scanner.Scan() {
			val := strings.TrimSpace(scanner.Text())
			if val != "" {
				pCfg.Model = val
			}
		}

	case "lmstudio":
		fmt.Print("Enter OpenAI-compatible Endpoint URL [default: http://localhost:1234/v1]: ")
		if scanner.Scan() {
			val := strings.TrimSpace(scanner.Text())
			if val != "" {
				pCfg.URL = val
			}
		}
		fmt.Print("Enter OpenAI-compatible Model [default: meta-llama-3-8b-instruct]: ")
		if scanner.Scan() {
			val := strings.TrimSpace(scanner.Text())
			if val != "" {
				pCfg.Model = val
			}
		}
		fmt.Print("Enter OpenAI-compatible API Key (Optional): ")
		if scanner.Scan() {
			val := strings.TrimSpace(scanner.Text())
			if val != "" {
				pCfg.APIKey = val
			}
		}
	}

	cfg.Providers[provider] = pCfg

	if err := config.SaveConfig(cfg); err != nil {
		fmt.Printf("❌ Failed to save setup config: %v\n", err)
	} else {
		fmt.Println("\n✅ Setup complete! Settings saved.")
	}
}

func handleList(cfg *config.Config) {
	if len(cfg.Apps) == 0 {
		fmt.Println("No applications created yet. Build one with `promptyly create \"<prompt>\"`!")
		return
	}

	fmt.Println("Saved Applications:")
	for name, path := range cfg.Apps {
		fmt.Printf("  - %-30s -> Path: %s\n", name, path)
	}
}

func handleURL(cfg *config.Config, urlString string) {
	if freshCfg, err := config.LoadConfig(); err == nil {
		cfg = freshCfg
	}

	if !strings.HasPrefix(urlString, "prompt://") {
		fmt.Printf("❌ Unsupported URL: %s (expected prompt://)\n", urlString)
		return
	}

	payload := strings.TrimPrefix(urlString, "prompt://")
	var targetVal string

	// Fallback/backward compatibility for legacy prompt://create?prompt=... format
	if strings.Contains(payload, "?") {
		u, err := url.Parse(urlString)
		if err == nil {
			if u.Host == "create" {
				targetVal = u.Query().Get("prompt")
			} else if u.Host == "run" || u.Host == "open" {
				targetVal = u.Query().Get("name")
			}
		}
	}

	if targetVal == "" {
		unescaped, err := url.QueryUnescape(payload)
		if err != nil {
			targetVal = payload
		} else {
			targetVal = unescaped
		}
	}

	targetVal = strings.TrimSpace(targetVal)
	if targetVal == "" {
		fmt.Println("❌ Empty prompt/name in URL.")
		return
	}

	// 1. Resolve application name by checking registry
	appName := ""
	targetSlug := app.Slugify(targetVal)

	if _, exists := cfg.Apps[targetVal]; exists {
		appName = targetVal
	} else if _, exists := cfg.Apps[targetSlug]; exists {
		appName = targetSlug
	} else {
		// 2. Check if it matches an associated prompt text
		for pr, name := range cfg.Prompts {
			if strings.EqualFold(pr, targetVal) {
				appName = name
				break
			}
		}
	}

	if appName != "" {
		fmt.Printf("🚀 Found existing application '%s'. Opening in browser...\n", appName)
		port := cfg.ServerPort
		if port == 0 {
			port = 6071
		}
		err := ensureServerRunning(port)
		if err != nil {
			fmt.Printf("❌ Failed to start background server: %v\n", err)
			os.Exit(1)
		}
		devURL := fmt.Sprintf("http://127.0.0.1:%d/apps/%s/", port, appName)
		app.OpenBrowser(devURL)
		time.Sleep(200 * time.Millisecond) // Give the OS command a moment to launch the browser
		os.Exit(0)
	} else {
		port := cfg.ServerPort
		if port == 0 {
			port = 6071
		}
		fmt.Printf("✨ Application not found for '%s'. Creating new application via Promptyly server...\n", targetVal)
		reqBody, _ := json.Marshal(map[string]string{"prompt": targetVal})
		appName, _, err := streamCreateRequest(port, reqBody)
		if err != nil {
			fmt.Printf("❌ Failed to create app: %v\n", err)
			return
		}
		if freshCfg, err := config.LoadConfig(); err == nil {
			cfg = freshCfg
		}
		err = app.InteractiveSession(cfg, appName)
		if err != nil {
			fmt.Printf("❌ Dev session failed: %v\n", err)
		}
	}
}

func getClientToken() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	tokenPath := filepath.Join(home, ".config", "promptyly", ".token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func ensureServerRunning(port int) error {
	// check if port is open
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return nil // Already running
	}

	// Start server in background
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(execPath, "serve")
	err = cmd.Start()
	if err != nil {
		return err
	}

	// Wait up to 3 seconds for server to start
	for i := 0; i < 30; i++ {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for background server to start")
}

func sendServerRequest(port int, method, path string, body []byte) ([]byte, error) {
	err := ensureServerRunning(port)
	if err != nil {
		return nil, fmt.Errorf("server is not running and could not be started: %v", err)
	}

	token := getClientToken()
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)

	var req *http.Request
	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Promptyly-Token", token)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func ensureAuthenticated(cfg *config.Config) (string, error) {
	serverURL := cfg.SharingServerURL
	if serverURL == "" {
		serverURL = "http://localhost:6072"
	}

	token := cfg.SharingToken
	if token != "" {
		return token, nil
	}

	fmt.Println("\nSharing Registry Server URL:", serverURL)
	fmt.Println("Sharing API token not configured. Please choose authentication option:")
	fmt.Println("1) Sign in with existing account")
	fmt.Println("2) Register a new account")
	fmt.Println("3) Provide API token directly")
	choice := promptInput("Select option (1-3): ")

	var err error
	switch choice {
	case "1":
		username := promptInput("Username: ")
		password := promptInput("Password: ")
		token, err = loginRequest(serverURL, username, password)
		if err != nil {
			return "", fmt.Errorf("login failed: %v", err)
		}
		cfg.SharingToken = token
		_ = config.SaveConfig(cfg)
		fmt.Println("✅ Sign in successful and token saved!")
	case "2":
		username := promptInput("Username: ")
		password := promptInput("Password (min 6 chars): ")
		token, err = registerRequest(serverURL, username, password)
		if err != nil {
			return "", fmt.Errorf("registration failed: %v", err)
		}
		cfg.SharingToken = token
		_ = config.SaveConfig(cfg)
		fmt.Println("✅ Account created and token saved!")
	case "3":
		token = promptInput("Paste sharing token: ")
		if token == "" {
			return "", fmt.Errorf("token cannot be empty")
		}
		cfg.SharingToken = token
		_ = config.SaveConfig(cfg)
		fmt.Println("✅ Token saved!")
	default:
		return "", fmt.Errorf("invalid option selected")
	}

	return token, nil
}

func getUsernameFromToken(serverURL, token string) (string, error) {
	u := fmt.Sprintf("%s/api/auth/me", strings.TrimSuffix(serverURL, "/"))
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Success  bool   `json:"success"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Username, nil
}

type RegistryAppInfo struct {
	Name     string `json:"name"`
	Username string `json:"username"`
}

func fetchRemoteAppsList(serverURL string) ([]RegistryAppInfo, error) {
	u := fmt.Sprintf("%s/api/apps/list", strings.TrimSuffix(serverURL, "/"))
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var list []RegistryAppInfo
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	return list, nil
}

func selectAppInteractively(cfg *config.Config, port int) (string, error) {
	token, err := ensureAuthenticated(cfg)
	if err != nil {
		return "", err
	}

	serverURL := cfg.SharingServerURL
	if serverURL == "" {
		serverURL = "http://localhost:6072"
	}

	// 1. Get logged-in username
	fmt.Println("Fetching account details from registry...")
	username, err := getUsernameFromToken(serverURL, token)
	if err != nil {
		return "", fmt.Errorf("failed to fetch user profile: %v (verify your sharing token/server URL)", err)
	}

	// 2. Fetch remote apps
	fmt.Println("Fetching application list from remote registry...")
	remoteApps, err := fetchRemoteAppsList(serverURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch registry apps: %v", err)
	}

	// Map remote app names published by this user
	publishedMap := make(map[string]bool)
	for _, app := range remoteApps {
		if strings.EqualFold(app.Username, username) {
			publishedMap[strings.ToLower(app.Name)] = true
		}
	}

	// 3. Sort local apps into published and unpublished
	var unpublished []string
	var published []string

	for name := range cfg.Apps {
		if publishedMap[strings.ToLower(name)] {
			published = append(published, name)
		} else {
			unpublished = append(unpublished, name)
		}
	}

	sort.Strings(unpublished)
	sort.Strings(published)

	if len(unpublished) == 0 && len(published) == 0 {
		return "", fmt.Errorf("no local applications found in your configuration")
	}

	fmt.Printf("\n==================================================\n")
	fmt.Printf("📢 Promptyly Publish Manager\n")
	fmt.Printf("==================================================\n")
	fmt.Printf("Logged in as: %s\n", username)
	fmt.Printf("Sharing Registry: %s\n\n", serverURL)

	index := 1
	choicesMap := make(map[int]string)

	fmt.Println("Unpublished Applications:")
	if len(unpublished) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, name := range unpublished {
			fmt.Printf("  [%d] %s\n", index, name)
			choicesMap[index] = name
			index++
		}
	}

	fmt.Println("\nAlready Published Applications (Select to update):")
	if len(published) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, name := range published {
			fmt.Printf("  [%d] %s\n", index, name)
			choicesMap[index] = name
			index++
		}
	}

	fmt.Printf("\nSelect an application number to publish/update (1-%d) [cancel]: ", index-1)
	choiceStr := promptInput("")
	if choiceStr == "" {
		return "", nil
	}

	choiceNum, err := strconv.Atoi(choiceStr)
	if err != nil || choiceNum < 1 || choiceNum >= index {
		return "", fmt.Errorf("invalid choice: %s", choiceStr)
	}

	selectedApp := choicesMap[choiceNum]
	return selectedApp, nil
}

func sendServerStreamRequest(port int, method, path string, body []byte, onChunk func(map[string]interface{})) error {
	err := ensureServerRunning(port)
	if err != nil {
		return fmt.Errorf("server is not running and could not be started: %v", err)
	}

	token := getClientToken()
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)

	var req *http.Request
	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Promptyly-Token", token)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(respBody))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(line), &chunk); err == nil {
			onChunk(chunk)
		}
	}

	return nil
}

func streamCreateRequest(port int, reqBody []byte) (string, string, error) {
	var finalResult struct {
		Success bool
		AppName string
		AppPath string
		Error   string
	}

	totalTokens := 0
	startTime := time.Now()
	intervalTokens := 0
	intervalStartTime := time.Now()
	var history []float64

	err := sendServerStreamRequest(port, "POST", "/api/apps/create", reqBody, func(chunk map[string]interface{}) {
		t, ok := chunk["type"].(string)
		if !ok {
			return
		}

		if t == "token" {
			totalTokens++
			intervalTokens++

			elapsedInterval := time.Since(intervalStartTime)
			if elapsedInterval >= 5*time.Second {
				tps := float64(intervalTokens) / elapsedInterval.Seconds()
				history = append(history, tps)
				if len(history) > 12 {
					history = history[1:]
				}
				intervalTokens = 0
				intervalStartTime = time.Now()
			}

			elapsedOverall := time.Since(startTime).Seconds()
			overallTPS := 0.0
			if elapsedOverall > 0 {
				overallTPS = float64(totalTokens) / elapsedOverall
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("\rTokens Generated: %d tokens  |  Speed: %.1f tokens/sec\n", totalTokens, overallTPS))
			sb.WriteString(app.DrawTPSGraph(history))

			if totalTokens > 1 {
				fmt.Print("\033[9A")
			}
			fmt.Print(sb.String())

		} else if t == "error" {
			finalResult.Error, _ = chunk["error"].(string)
		} else if t == "success" {
			finalResult.Success = true
			finalResult.AppName, _ = chunk["appName"].(string)
			finalResult.AppPath, _ = chunk["appPath"].(string)
		}
	})

	if err != nil {
		return "", "", err
	}

	if finalResult.Error != "" {
		return "", "", fmt.Errorf(finalResult.Error)
	}

	return finalResult.AppName, finalResult.AppPath, nil
}

