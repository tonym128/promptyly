package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"promptyly/app"
	"promptyly/config"
	"promptyly/server"
	"promptyly/urlscheme"
	"strconv"
	"strings"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Register API callbacks for the background daemon
	server.CreateAppCallback = func(prompt string) (string, string, error) {
		freshCfg, err := config.LoadConfig()
		if err != nil {
			return "", "", fmt.Errorf("failed to reload config: %v", err)
		}
		return app.CreateApp(freshCfg, prompt)
	}
	server.EditAppCallback = func(name, prompt string) error {
		freshCfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to reload config: %v", err)
		}
		return app.EditApp(freshCfg, name, prompt)
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

	command := strings.ToLower(os.Args[1])

	switch command {
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
		appName, _, err := app.CreateApp(cfg, promptVal)
		if err != nil {
			fmt.Printf("❌ Creation failed: %v\n", err)
			os.Exit(1)
		}
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
		err = app.InteractiveSession(cfg, appName)
		if err != nil {
			fmt.Printf("❌ Run session failed: %v\n", err)
			os.Exit(1)
		}

	case "list":
		handleList(cfg)

	case "export":
		if len(os.Args) < 4 {
			fmt.Println("Usage: promptyly export <app-name> <output.zip>")
			return
		}
		appName := os.Args[2]
		zipPath := os.Args[3]
		err := app.ExportApp(cfg, appName, zipPath)
		if err != nil {
			fmt.Printf("❌ Export failed: %v\n", err)
			os.Exit(1)
		}

	case "import":
		if len(os.Args) < 3 {
			fmt.Println("Usage: promptyly import <zip-path>")
			return
		}
		zipPath := os.Args[2]
		appName, err := app.ImportApp(cfg, zipPath)
		if err != nil {
			fmt.Printf("❌ Import failed: %v\n", err)
			os.Exit(1)
		}
		// Ask if they want to run it right away
		fmt.Printf("Would you like to run '%s' now? (y/n) [y]: ", appName)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
			if ans == "" || ans == "y" || ans == "yes" {
				err = app.InteractiveSession(cfg, appName)
				if err != nil {
					fmt.Printf("❌ Run session failed: %v\n", err)
					os.Exit(1)
				}
			}
		}

	case "serve", "daemon":
		port := cfg.ServerPort
		if port == 0 {
			port = 6071
		}
		_, err := server.StartDevServer(port)
		if err != nil {
			fmt.Printf("❌ Failed to start server: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("🚀 Promptyly background server/API running on http://127.0.0.1:%d\n", port)
		fmt.Println("Press Ctrl+C to exit.")
		select {}

	case "publish":
		if len(os.Args) < 3 {
			fmt.Println("Usage: promptyly publish <app-name>")
			return
		}
		appName := os.Args[2]
		err := PublishApp(cfg, appName)
		if err != nil {
			fmt.Printf("❌ Publish failed: %v\n", err)
			os.Exit(1)
		}

	case "search":
		if len(os.Args) < 3 {
			fmt.Println("Usage: promptyly search \"<query>\"")
			return
		}
		query := os.Args[2]
		err := SearchApps(cfg, query)
		if err != nil {
			fmt.Printf("❌ Search failed: %v\n", err)
			os.Exit(1)
		}

	case "download":
		if len(os.Args) < 3 {
			fmt.Println("Usage: promptyly download <app-id>")
			return
		}
		appID := os.Args[2]
		err := DownloadApp(cfg, appID)
		if err != nil {
			fmt.Printf("❌ Download failed: %v\n", err)
			os.Exit(1)
		}

	case "register":
		err := urlscheme.Register()
		if err != nil {
			fmt.Printf("❌ Registration failed: %v\n", err)
			os.Exit(1)
		}

	case "unregister":
		err := urlscheme.Unregister()
		if err != nil {
			fmt.Printf("❌ Unregistration failed: %v\n", err)
			os.Exit(1)
		}

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
	fmt.Printf("Default LLM Provider: %s\n", cfg.DefaultProvider)
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
		fmt.Printf("  - %-10s -> Model: %-30s | Status: %s%s\n", k, v.Model, status, urlStr)
	}
}

func handleConfigSet(cfg *config.Config, key, val string) {
	switch strings.ToLower(key) {
	case "default_provider":
		cfg.DefaultProvider = val
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
	case "lmstudio_url":
		p := cfg.Providers["lmstudio"]
		p.URL = val
		cfg.Providers["lmstudio"] = p
	case "lmstudio_model":
		p := cfg.Providers["lmstudio"]
		p.Model = val
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
	fmt.Println("Configure credentials for LLM providers (Gemini, Claude, Ollama, LM Studio).")
	fmt.Println("--------------------------------------------------")

	// 1. Choose Provider
	fmt.Println("Select default LLM provider:")
	fmt.Println("1) Gemini (Recommended - Google)")
	fmt.Println("2) Claude (Anthropic)")
	fmt.Println("3) Ollama (Local LLM Server)")
	fmt.Println("4) LM Studio (Local OpenAI-Compatible Server)")
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
		fmt.Print("Enter LM Studio Endpoint URL [default: http://localhost:1234/v1]: ")
		if scanner.Scan() {
			val := strings.TrimSpace(scanner.Text())
			if val != "" {
				pCfg.URL = val
			}
		}
		fmt.Print("Enter LM Studio Model [default: meta-llama-3-8b-instruct]: ")
		if scanner.Scan() {
			val := strings.TrimSpace(scanner.Text())
			if val != "" {
				pCfg.Model = val
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
		fmt.Printf("🚀 Found existing application '%s'. Running it...\n", appName)
		err := app.InteractiveSession(cfg, appName)
		if err != nil {
			fmt.Printf("❌ Run session failed: %v\n", err)
		}
	} else {
		fmt.Printf("✨ Application not found for '%s'. Creating new application...\n", targetVal)
		newName, _, err := app.CreateApp(cfg, targetVal)
		if err != nil {
			fmt.Printf("❌ Failed to create app: %v\n", err)
			return
		}
		err = app.InteractiveSession(cfg, newName)
		if err != nil {
			fmt.Printf("❌ Dev session failed: %v\n", err)
		}
	}
}
