package app

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"promptyly/agent"
	"promptyly/config"
	"promptyly/git"
	"promptyly/history"
	"promptyly/server"
	"runtime"
	"strings"
	"time"
)

// Slugify converts a text prompt into a clean URL-friendly/directory-friendly name.
func Slugify(s string) string {
	s = strings.ToLower(s)
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			if sb.Len() > 0 && sb.String()[sb.Len()-1] != '-' {
				sb.WriteRune('-')
			}
		}
	}
	res := sb.String()
	res = strings.Trim(res, "-")
	if len(res) > 30 {
		res = res[:30]
		res = strings.TrimSuffix(res, "-")
	}
	if len(res) == 0 {
		res = "app"
	}
	return res
}

func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // "linux"
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}


// CreateApp generates a new application from a prompt.
func CreateApp(ctx context.Context, cfg *config.Config, prompt string, onToken func(token string)) (string, string, error) {
	trimmedPrompt := strings.TrimSpace(prompt)
	if existingAppName, exists := cfg.Prompts[trimmedPrompt]; exists {
		if appDir, dirExists := cfg.Apps[existingAppName]; dirExists {
			if _, err := os.Stat(appDir); err == nil {
				fmt.Printf("Found existing application '%s' for this prompt.\nOpening it instead of generating a new one...\n", existingAppName)
				return existingAppName, appDir, nil
			} else {
				// Cleanup stale entries
				delete(cfg.Prompts, trimmedPrompt)
				delete(cfg.Apps, existingAppName)
				_ = config.SaveConfig(cfg)
			}
		}
	}

	appName := Slugify(prompt)
	appDir := filepath.Join(cfg.AppsDir, appName)

	// Check remote sharing registry first if configured
	if cfg.CheckRemoteFirst {
		importedName, err := checkAndDownloadRemote(cfg, appName)
		if err == nil && importedName != "" {
			fmt.Printf("✅ Reused remote application '%s' successfully!\n", importedName)
			return importedName, cfg.Apps[importedName], nil
		} else if err != nil {
			fmt.Printf("Warning: Remote app search failed: %v. Proceeding with local generation.\n", err)
		}
	}

	// Avoid overwriting existing apps by appending timestamps if directory exists
	if _, err := os.Stat(appDir); err == nil {
		appName = fmt.Sprintf("%s-%d", appName, time.Now().Unix()%1000)
		appDir = filepath.Join(cfg.AppsDir, appName)
	}

	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create directory: %v", err)
	}

	// Init Git
	if err := git.Init(appDir); err != nil {
		return "", "", fmt.Errorf("failed to initialize git repo: %v", err)
	}

	// Set up agent client
	prov, provCfg := cfg.GetActiveProvider()
	client, err := agent.NewClient(prov, provCfg)
	if err != nil {
		return "", "", err
	}

	// Write app-specific config to store the application configuration as a separate setup
	localConfigDir := filepath.Join(appDir, ".promptyly")
	_ = os.MkdirAll(localConfigDir, 0755)
	localConfigPath := filepath.Join(localConfigDir, "config.json")
	appConfigData := map[string]string{
		"provider": prov,
		"model":    provCfg.Model,
	}
	if configBytes, err := json.MarshalIndent(appConfigData, "", "  "); err == nil {
		_ = os.WriteFile(localConfigPath, configBytes, 0644)
	}
	if err != nil {
		return "", "", err
	}

	systemPrompt := `You are an expert Frontend Web Developer AI.
Your task is to build a fully functional, self-contained, responsive, and visually stunning web application based on the user's prompt.
You must return your output using specific XML tags:
For each code file, wrap it in:
<file name="filename">
... code ...
</file>

For a concise summary of your changes, wrap it in:
<summary>
... summary ...
</summary>

Follow these guidelines:
1. Provide a modern, clean, and premium user experience (rich aesthetics, custom font pairings, dark mode or clean themes, smooth animations, cards, glassmorphism, responsive grids).
2. Avoid placeholders; all logic must be fully written and ready to run.
3. Use only client-side files (HTML, CSS, JS). You can inject external libraries like Tailwind CSS (via CDN) or Google Fonts, FontAwesome, or Lucide icons if needed.
4. For persistent data, you can read/write by making fetch calls to '_promptyly/api/db' (relative to the current path).
   - GET _promptyly/api/db returns the stored JSON object.
   - POST _promptyly/api/db saves the JSON object (send body as content-type application/json).
   Use this endpoint if the application requires state persistence (e.g. keeping notes, todos, etc.) so that it survives reloads.
`

	fmt.Printf("Generating application '%s' using provider '%s' (model: %s)...\n", appName, prov, provCfg.Model)
	resp, err := client.Generate(ctx, systemPrompt, prompt, onToken)
	if err != nil {
		return "", "", err
	}

	if len(resp.Files) == 0 {
		return "", "", fmt.Errorf("agent failed to generate any files. Raw summary: %s", resp.Summary)
	}

	// Write files
	var filesWritten []string
	for filename, content := range resp.Files {
		fullPath := filepath.Join(appDir, filename)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return "", "", err
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return "", "", err
		}
		filesWritten = append(filesWritten, filename)
		fmt.Printf("  Created: %s (%d bytes)\n", filename, len(content))
	}

	// Generate README.md
	readmePath := filepath.Join(appDir, "README.md")
	displayName := strings.Title(strings.ReplaceAll(appName, "-", " "))
	readmeContentRaw := `# %s

Generated by **Promptyly** based on prompt:
> "%s"

## Getting Started
To run this application locally, use the Promptyly CLI or desktop app:
~bash
promptyly handle "prompt://%s"
~

## Structure
- ~index.html~: Core application HTML and client-side logic.
- ~README.md~: Project overview.
- ~AGENT.md~: Instructions for AI agents during future edit sessions.
`
	readmeContent := strings.ReplaceAll(fmt.Sprintf(readmeContentRaw, displayName, prompt, appName), "~", "`")
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err == nil {
		filesWritten = append(filesWritten, "README.md")
		fmt.Println("  Created: README.md")
	}

	// Generate AGENT.md
	agentPath := filepath.Join(appDir, "AGENT.md")
	agentContentRaw := `# Agent Instructions & Project Context

You are editing the application: **%s**.

## Original Generation Prompt
> "%s"

## Guidelines for Future Edits
- **Self-Contained**: Keep the application client-side. Utilize HTML, inline or external CSS/JS, and CDNs (Tailwind CSS, Lucide icons, etc.) if already present.
- **State Persistence**: A dynamic storage API is available. Read/write state by making fetch calls relative to the app:
  - ~GET _promptyly/api/db~ (returns stored JSON object)
  - ~POST _promptyly/api/db~ (saves JSON object; send body as JSON)
- **Git Backed**: Keep commits meaningful and follow the prompt guidelines.
`
	agentContent := strings.ReplaceAll(fmt.Sprintf(agentContentRaw, appName, prompt), "~", "`")
	if err := os.WriteFile(agentPath, []byte(agentContent), 0644); err == nil {
		filesWritten = append(filesWritten, "AGENT.md")
		fmt.Println("  Created: AGENT.md")
	}

	// Save history
	hEntry := history.ActionEntry{
		Action:        "create",
		Prompt:        prompt,
		Provider:      prov,
		Model:         provCfg.Model,
		FilesAffected: filesWritten,
		Summary:       resp.Summary,
	}
	if err := history.AddEntry(appDir, hEntry); err != nil {
		fmt.Printf("Warning: failed to save history: %v\n", err)
	}

	// Commit Git
	commitMsg := fmt.Sprintf("Initialize application: %s\n\nAI Summary: %s", prompt, resp.Summary)
	if _, err := git.CommitAll(appDir, commitMsg); err != nil {
		fmt.Printf("Warning: git commit failed: %v\n", err)
	}

	// Save to config registry
	cfg.Apps[appName] = appDir
	cfg.Prompts[trimmedPrompt] = appName
	_ = config.SaveConfig(cfg)

	return appName, appDir, nil
}

// EditApp processes a modification request for an existing application.
func EditApp(ctx context.Context, cfg *config.Config, appName string, editPrompt string, onToken func(token string)) error {
	appDir := cfg.ResolveAppPath(appName)
	if appDir == "" {
		// Try resolving as direct directory
		if _, err := os.Stat(appName); err == nil {
			appDir = appName
			appName = filepath.Base(appName)
		} else {
			return fmt.Errorf("app '%s' not registered and not found as a path", appName)
		}
	}

	// Read current directory code files to pass as context
	filesContext := strings.Builder{}
	err := filepath.Walk(appDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip .git, .promptyly, and node_modules if any
			name := info.Name()
			if name == ".git" || name == ".promptyly" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(appDir, path)
		if err != nil {
			return err
		}

		// Only include text code files
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == ".html" || ext == ".css" || ext == ".js" || ext == ".json" {
			data, err := os.ReadFile(path)
			if err == nil {
				filesContext.WriteString(fmt.Sprintf("\n--- FILE: %s ---\n%s\n", rel, string(data)))
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to read existing codebase: %v", err)
	}

	prov, provCfg := ResolveAppProvider(appDir, cfg)
	client, err := agent.NewClient(prov, provCfg)
	if err != nil {
		return err
	}

	systemPrompt := `You are an expert Frontend Web Developer AI.
Your task is to modify the existing web application based on the user's edit request.
The current code files in the directory are:
` + filesContext.String() + `

Return ONLY the files you modified or created. You do not need to return files that remain unchanged.
Return your output using specific XML tags:
For each code file, wrap it in:
<file name="filename">
... code ...
</file>

For a concise summary of your changes, wrap it in:
<summary>
... summary ...
</summary>

Rules:
1. Return the entire contents of the files you modified. Do not send partial files or placeholder comments like "// no changes here".
2. Maintain existing styles, behaviors, and structure unless explicitly asked to modify them.
3. Keep the design premium, responsive, and clean.
4. For data persistence, use '_promptyly/api/db' (GET to fetch, POST with body to save, relative to the current path) if applicable.
`

	fmt.Printf("Applying edits using provider '%s' (model: %s)...\n", prov, provCfg.Model)
	resp, err := client.Generate(ctx, systemPrompt, editPrompt, onToken)
	if err != nil {
		return err
	}

	if len(resp.Files) == 0 {
		return fmt.Errorf("agent did not suggest any changes. Raw summary: %s", resp.Summary)
	}

	var filesWritten []string
	for filename, content := range resp.Files {
		fullPath := filepath.Join(appDir, filename)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
		filesWritten = append(filesWritten, filename)
		fmt.Printf("  Updated: %s (%d bytes)\n", filename, len(content))
	}

	// Save history
	hEntry := history.ActionEntry{
		Action:        "edit",
		Prompt:        editPrompt,
		Provider:      prov,
		Model:         provCfg.Model,
		FilesAffected: filesWritten,
		Summary:       resp.Summary,
	}
	if err := history.AddEntry(appDir, hEntry); err != nil {
		fmt.Printf("Warning: failed to save history: %v\n", err)
	}

	// Commit Git
	commitMsg := fmt.Sprintf("Edit application: %s\n\nAI Summary: %s", editPrompt, resp.Summary)
	if _, err := git.CommitAll(appDir, commitMsg); err != nil {
		fmt.Printf("Warning: git commit failed: %v\n", err)
	}

	return nil
}

// triggerReload sends a reload webhook POST request to the local server.
func triggerReload(appName string, port int) {
	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/apps/%s/_promptyly/reload", port, appName), "application/json", nil)
	if err == nil {
		_ = resp.Body.Close()
	}
}

// InteractiveSession runs the app web server and enters a CLI prompt loop for real-time edits.
func InteractiveSession(cfg *config.Config, appName string) error {
	appDir, ok := cfg.Apps[appName]
	if !ok {
		// Resolve direct path
		if _, err := os.Stat(appName); err == nil {
			appDir = appName
			appName = filepath.Base(appName)
		} else {
			return fmt.Errorf("app '%s' not registered and not found as a path", appName)
		}
	}

	port := cfg.ServerPort
	if port == 0 {
		port = 6071
	}

	_, err := server.StartDevServer(port)
	if err != nil {
		return fmt.Errorf("failed to start dev server: %v", err)
	}

	devURL := fmt.Sprintf("http://127.0.0.1:%d/apps/%s/", port, appName)
	fmt.Printf("\n=========================================\n")
	fmt.Printf("🚀 App '%s' is running!\n", appName)
	fmt.Printf("👉 URL: %s\n", devURL)
	fmt.Printf("📁 Path: %s\n", appDir)
	fmt.Printf("=========================================\n\n")

	// Check for updates and automatically update in the background
	go func() {
		if res, err := config.CheckForUpdates(cfg.SharingServerURL); err == nil && res.IsNewer {
			fmt.Printf("\n✨ UPDATE AVAILABLE: Version v%s is now available on the registry (you are running v%s)!\n", res.ServerVersion, config.Version)
			fmt.Printf("⚙️ Automatically downloading and installing update (v%s) in the background...\n", res.ServerVersion)
			if err := config.TriggerSelfUpdate(cfg.SharingServerURL); err == nil {
				fmt.Printf("\n✅ Promptyly successfully updated to v%s! Changes will take effect on next run.\n\n", res.ServerVersion)
			} else {
				fmt.Printf("\n⚠️ Automatic update failed: %v. You can update manually using the installer or visit: %s\n\n", err, cfg.SharingServerURL)
			}
		}
	}()

	OpenBrowser(devURL)

	fmt.Println("Interactive editing mode active.")
	fmt.Println("Describe changes you want to make (e.g., 'make the font larger', 'add a dark mode').")
	fmt.Println("Available commands:")
	fmt.Println("  .publish [desc] - Publish the app to the remote registry (optionally with description)")
	fmt.Println("  .reload         - Manually trigger browser hot-reload")
	fmt.Println("  .exit           - Stop the dev server and exit interactive mode (or type 'exit')")
	fmt.Println("-----------------------------------------")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("promptyly> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == ".exit" || strings.ToLower(input) == "exit" {
			fmt.Println("Stopping server. Goodbye!")
			break
		}

		if input == ".reload" {
			fmt.Println("🔄 Triggering browser hot-reload...")
			triggerReload(appName, port)
			fmt.Println("✅ Hot-reload triggered!")
			fmt.Println()
			continue
		}

		if input == ".llm" || strings.HasPrefix(input, ".llm ") {
			handleLlmCommand(cfg, appName, input)
			continue
		}

		if input == ".publish" || strings.HasPrefix(input, ".publish ") {
			var desc string
			if input == ".publish" {
				fmt.Print("Enter an optional description: ")
				if !scanner.Scan() {
					break
				}
				desc = strings.TrimSpace(scanner.Text())
			} else {
				desc = strings.TrimSpace(strings.TrimPrefix(input, ".publish "))
			}
			fmt.Printf("Publishing application '%s' via Promptyly server...\n", appName)
			if err := sendServerPublishRequest(port, appName, desc); err != nil {
				fmt.Printf("❌ Publish failed: %v\n\n", err)
			}
			continue
		}

		fmt.Printf("\n[AI Working...] Processing request: '%s'\n", input)
		if err := sendServerEditRequest(port, appName, input); err != nil {
			fmt.Printf("❌ Error: %v\n\n", err)
		} else {
			fmt.Println("✅ Changes applied and committed to git!")
			fmt.Println("🔄 Triggering browser hot-reload...")
			triggerReload(appName, port)
			fmt.Println()
		}
	}

	return nil
}

func sendServerPublishRequest(port int, appName, description string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	tokenPath := filepath.Join(home, ".config", "promptyly", ".token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return fmt.Errorf("API token not found, is server running? error: %v", err)
	}
	token := strings.TrimSpace(string(data))

	url := fmt.Sprintf("http://127.0.0.1:%d/api/apps/publish", port)
	reqBody, _ := json.Marshal(map[string]string{
		"name":        appName,
		"description": description,
	})

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Promptyly-Token", token)

	client := &http.Client{Timeout: 60 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var respData struct {
		Success   bool   `json:"success"`
		AppID     string `json:"appId"`
		LiveURL   string `json:"liveUrl"`
		DetailURL string `json:"detailUrl"`
	}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	fmt.Printf("\n==================================================\n")
	fmt.Printf("✅ Application successfully published to the registry!\n")
	fmt.Printf("👉 Live URL: %s\n", respData.LiveURL)
	fmt.Printf("👉 Detail Page: %s\n", respData.DetailURL)
	fmt.Printf("==================================================\n\n")

	return nil
}

// ExportApp creates a zip file of the application.
func ExportApp(cfg *config.Config, appName, outputPath string) error {
	appDir := cfg.ResolveAppPath(appName)
	if appDir == "" {
		if _, err := os.Stat(appName); err == nil {
			appDir = appName
		} else {
			return fmt.Errorf("app '%s' not found", appName)
		}
	}

	zipFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	err = filepath.Walk(appDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Exclude git history in export to keep zip lightweight, but keep .promptyly configuration/history
		relPath, err := filepath.Rel(appDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		if strings.HasPrefix(relPath, ".git") {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(relPath)
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to zip directory: %v", err)
	}

	fmt.Printf("Successfully exported '%s' to %s\n", appName, outputPath)
	return nil
}

// ImportApp extracts a zipped application and registers it locally.
func ImportApp(cfg *config.Config, zipPath string) (string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	baseName := strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
	appName := Slugify(baseName)
	appDir := filepath.Join(cfg.AppsDir, appName)

	if _, err := os.Stat(appDir); err == nil {
		appName = fmt.Sprintf("%s-imported-%d", appName, time.Now().Unix()%1000)
		appDir = filepath.Join(cfg.AppsDir, appName)
	}

	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}

	for _, file := range reader.File {
		path := filepath.Join(appDir, file.Name)
		if file.FileInfo().IsDir() {
			_ = os.MkdirAll(path, file.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", err
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return "", err
		}

		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return "", err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return "", err
		}
	}

	// Initialize git repository in the imported app folder if not present
	if _, err := os.Stat(filepath.Join(appDir, ".git")); os.IsNotExist(err) {
		_ = git.Init(appDir)
		_, _ = git.CommitAll(appDir, "Imported project from zip archive")
	}

	// Register in config
	cfg.Apps[appName] = appDir
	_ = config.SaveConfig(cfg)

	fmt.Printf("Successfully imported application as '%s' to %s\n", appName, appDir)
	return appName, nil
}

// RenameApp renames an application directory and updates the config registry.
func RenameApp(cfg *config.Config, oldName, newName string) (string, error) {
	oldPath, ok := cfg.Apps[oldName]
	if !ok {
		return "", fmt.Errorf("app '%s' not found", oldName)
	}

	slugName := Slugify(newName)
	if slugName == oldName {
		return oldName, nil
	}

	if _, exists := cfg.Apps[slugName]; exists {
		return "", fmt.Errorf("app '%s' already exists", slugName)
	}

	newPath := filepath.Join(filepath.Dir(oldPath), slugName)
	if err := os.Rename(oldPath, newPath); err != nil {
		return "", fmt.Errorf("failed to rename directory: %v", err)
	}

	// Update registry
	delete(cfg.Apps, oldName)
	cfg.Apps[slugName] = newPath

	// Update prompts mapping
	for pr, name := range cfg.Prompts {
		if name == oldName {
			cfg.Prompts[pr] = slugName
		}
	}

	if err := config.SaveConfig(cfg); err != nil {
		return "", fmt.Errorf("failed to save config: %v", err)
	}

	return slugName, nil
}

// LinkApp links an existing directory as a registered application.
func LinkApp(cfg *config.Config, path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %v", err)
	}

	fi, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("path does not exist: %v", err)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}

	// Verify it contains index.html
	if _, err := os.Stat(filepath.Join(absPath, "index.html")); os.IsNotExist(err) {
		return "", fmt.Errorf("directory does not contain index.html")
	}

	appName := Slugify(filepath.Base(absPath))
	// Avoid collisions
	baseName := appName
	counter := 1
	for {
		if _, exists := cfg.Apps[appName]; !exists {
			break
		}
		appName = fmt.Sprintf("%s-%d", baseName, counter)
		counter++
	}

	cfg.Apps[appName] = absPath
	if err := config.SaveConfig(cfg); err != nil {
		return "", fmt.Errorf("failed to save config: %v", err)
	}

	return appName, nil
}

// UnlinkApp unregisters an application from the registry without deleting its folder.
func UnlinkApp(cfg *config.Config, appName string) error {
	if _, ok := cfg.Apps[appName]; !ok {
		return fmt.Errorf("app '%s' not found", appName)
	}

	delete(cfg.Apps, appName)

	// Clean prompts mapping
	for pr, name := range cfg.Prompts {
		if name == appName {
			delete(cfg.Prompts, pr)
		}
	}

	return config.SaveConfig(cfg)
}

// DeleteApp deletes the application directory and removes it from the registry.
func DeleteApp(cfg *config.Config, appName string) error {
	appPath := cfg.ResolveAppPath(appName)
	if appPath == "" {
		return fmt.Errorf("app '%s' not found", appName)
	}

	// Remove from registry first to prevent stale references if delete fails partially
	delete(cfg.Apps, appName)
	for pr, name := range cfg.Prompts {
		if name == appName {
			delete(cfg.Prompts, pr)
		}
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %v", err)
	}

	// Delete folder
	if err := os.RemoveAll(appPath); err != nil {
		return fmt.Errorf("failed to delete directory: %v", err)
	}

	return nil
}

func checkAndDownloadRemote(cfg *config.Config, appName string) (string, error) {
	serverURL := cfg.SharingServerURL
	if serverURL == "" {
		serverURL = "http://localhost:6072"
	}

	u := fmt.Sprintf("%s/api/apps/search?q=%s", strings.TrimSuffix(serverURL, "/"), url.QueryEscape(appName))
	resp, err := http.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search failed with status %d", resp.StatusCode)
	}

	type RemoteApp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	var remoteApps []RemoteApp
	if err := json.NewDecoder(resp.Body).Decode(&remoteApps); err != nil {
		return "", err
	}

	var matchedApp *RemoteApp
	for _, app := range remoteApps {
		if strings.EqualFold(app.ID, appName) {
			matchedApp = &app
			break
		}
	}

	if matchedApp == nil {
		return "", nil
	}

	fmt.Printf("✨ Found matching application '%s' in remote sharing registry. Downloading...\n", matchedApp.ID)
	downloadURL := fmt.Sprintf("%s/api/apps/download/%s", strings.TrimSuffix(serverURL, "/"), matchedApp.ID)
	dlResp, err := http.Get(downloadURL)
	if err != nil {
		return "", err
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", dlResp.StatusCode)
	}

	tempZip := filepath.Join(os.TempDir(), fmt.Sprintf("promptyly-remote-dl-%s.zip", matchedApp.ID))
	out, err := os.Create(tempZip)
	if err != nil {
		return "", err
	}
	defer os.Remove(tempZip)

	if _, err := io.Copy(out, dlResp.Body); err != nil {
		_ = out.Close()
		return "", err
	}
	_ = out.Close()

	importedName, err := ImportApp(cfg, tempZip)
	if err != nil {
		return "", err
	}

	return importedName, nil
}

func sendServerEditRequest(port int, appName, prompt string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	tokenPath := filepath.Join(home, ".config", "promptyly", ".token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return fmt.Errorf("API token not found, is server running? error: %v", err)
	}
	token := strings.TrimSpace(string(data))

	url := fmt.Sprintf("http://127.0.0.1:%d/api/apps/edit", port)
	reqBody, _ := json.Marshal(map[string]string{
		"name":   appName,
		"prompt": prompt,
	})

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Promptyly-Token", token)

	client := &http.Client{Timeout: 60 * time.Minute}
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
	totalTokens := 0
	startTime := time.Now()
	intervalTokens := 0
	intervalStartTime := time.Now()
	var history []float64

	var finalResult struct {
		Success bool
		Error   string
	}

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
			t, ok := chunk["type"].(string)
			if !ok {
				continue
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

				// Include current interval in graph for immediate feedback
				var currentTPS float64
				elapsedSeconds := elapsedInterval.Seconds()
				if elapsedSeconds > 0 {
					currentTPS = float64(intervalTokens) / elapsedSeconds
				}
				tempHistory := append(history, currentTPS)
				if len(tempHistory) > 12 {
					tempHistory = tempHistory[1:]
				}

				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("\rTokens Generated: %d tokens  |  Speed: %.1f tokens/sec\n", totalTokens, overallTPS))
				sb.WriteString(DrawTPSGraph(tempHistory))

				if totalTokens > 1 {
					fmt.Print("\033[9A")
				}
				fmt.Print(sb.String())

			} else if t == "error" {
				finalResult.Error, _ = chunk["error"].(string)
			} else if t == "success" {
				finalResult.Success = true
			}
		}
	}

	if finalResult.Error != "" {
		return fmt.Errorf(finalResult.Error)
	}

	return nil
}

// DrawTPSGraph generates an ASCII/Unicode vertical bar graph representing tokens-per-second history.
func DrawTPSGraph(history []float64) string {
	maxVal := 0.0
	for _, v := range history {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal < 10 {
		maxVal = 10.0 // minimum scale
	}

	rows := 4
	var sb strings.Builder
	sb.WriteString("\nTP/S History (last 60s, 5s intervals):\n")

	for r := rows - 1; r >= 0; r-- {
		valAtRow := maxVal * float64(r) / float64(rows-1)
		sb.WriteString(fmt.Sprintf("%4.1f | ", valAtRow))

		for _, v := range history {
			h := (v / maxVal) * float64(rows)
			if h >= float64(r+1) {
				sb.WriteString("█  ")
			} else if h <= float64(r) {
				sb.WriteString("   ")
			} else {
				fraction := h - float64(r)
				if fraction < 0.125 {
					sb.WriteString("   ")
				} else if fraction < 0.25 {
					sb.WriteString("  ")
				} else if fraction < 0.375 {
					sb.WriteString("▂  ")
				} else if fraction < 0.5 {
					sb.WriteString("▃  ")
				} else if fraction < 0.625 {
					sb.WriteString("▄  ")
				} else if fraction < 0.75 {
					sb.WriteString("▅  ")
				} else if fraction < 0.875 {
					sb.WriteString("▆  ")
				} else {
					sb.WriteString("▇  ")
				}
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("     +------------------------------------\n")
	sb.WriteString("       -60s                             0s\n")
	return sb.String()
}

func ResolveAppProvider(appDir string, globalCfg *config.Config) (string, config.ProviderConfig) {
	localPath := filepath.Join(appDir, ".promptyly", "config.json")
	if _, err := os.Stat(localPath); err == nil {
		data, err := os.ReadFile(localPath)
		if err == nil {
			var local struct {
				Provider string `json:"provider"`
				Model    string `json:"model"`
			}
			if json.Unmarshal(data, &local) == nil {
				if local.Provider != "" {
					pCfg, ok := globalCfg.Providers[local.Provider]
					if ok {
						if local.Model != "" {
							pCfg.Model = local.Model
						}
						return local.Provider, pCfg
					}
				}
			}
		}
	}
	return globalCfg.GetActiveProvider()
}

func handleLlmCommand(cfg *config.Config, appName, input string) {
	appDir, ok := cfg.Apps[appName]
	if !ok {
		if _, err := os.Stat(appName); err == nil {
			appDir = appName
		} else {
			fmt.Printf("❌ App '%s' not registered\n", appName)
			return
		}
	}

	localPath := filepath.Join(appDir, ".promptyly", "config.json")
	var local struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}

	if data, err := os.ReadFile(localPath); err == nil {
		_ = json.Unmarshal(data, &local)
	}

	if local.Provider == "" {
		activeProv, activeProvCfg := cfg.GetActiveProvider()
		local.Provider = activeProv
		local.Model = activeProvCfg.Model
	}

	args := strings.Fields(input)
	if len(args) == 1 {
		fmt.Printf("\n--- LLM Configuration for App '%s' ---\n", appName)
		fmt.Printf("Active Provider: %s\n", local.Provider)
		fmt.Printf("Active Model:    %s\n", local.Model)
		fmt.Println("---------------------------------------")
		fmt.Println("Available Providers & Models (Global Config):")

		for prov, provCfg := range cfg.Providers {
			fmt.Printf("  - %s (%s):\n", prov, provCfg.URL)
			for _, m := range provCfg.Models {
				activeMarker := ""
				if prov == local.Provider && m == local.Model {
					activeMarker = " (active)"
				}
				fmt.Printf("      * %s%s\n", m, activeMarker)
			}
		}
		fmt.Println("\nTo change settings, use:")
		fmt.Println("  .llm <provider> [model]   - Set provider and optional model for this app")
		fmt.Println("  .llm download <model>     - Download a local llamafile model (qwen2.5-coder-1.5b-instruct, llama-3.2-1b-instruct)")
		fmt.Println()
		return
	}

	subcmd := strings.ToLower(args[1])
	if subcmd == "download" {
		if len(args) < 3 {
			fmt.Println("❌ Usage: .llm download <model-name>")
			return
		}
		modelName := args[2]
		err := downloadLlamafile(modelName)
		if err != nil {
			fmt.Printf("❌ Download failed: %v\n", err)
			return
		}
		if freshCfg, err := config.LoadConfig(); err == nil {
			*cfg = *freshCfg
		}
		return
	}

	provider := args[1]
	pCfg, exists := cfg.Providers[provider]
	if !exists {
		fmt.Printf("❌ Unknown provider: %s\n", provider)
		return
	}

	model := pCfg.Model
	if len(args) >= 3 {
		model = args[2]
		found := false
		for _, m := range pCfg.Models {
			if strings.ToLower(m) == strings.ToLower(model) {
				model = m
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("⚠️ Model '%s' is not in the default list for '%s', but setting it anyway.\n", model, provider)
		}
	}

	local.Provider = provider
	local.Model = model

	localConfigDir := filepath.Join(appDir, ".promptyly")
	_ = os.MkdirAll(localConfigDir, 0755)
	if configBytes, err := json.MarshalIndent(local, "", "  "); err == nil {
		if os.WriteFile(localPath, configBytes, 0644) == nil {
			fmt.Printf("✅ Updated LLM setup for App '%s': %s (model: %s)\n\n", appName, provider, model)
		} else {
			fmt.Println("❌ Failed to write local config file.")
		}
	}
}

func downloadLlamafile(modelName string) error {
	var url string
	var filename string

	switch strings.ToLower(modelName) {
	case "qwen2.5-coder-1.5b-instruct", "qwen", "qwen1.5b":
		url = "https://huggingface.co/Bojun-Feng/Qwen2.5-Coder-1.5B-Instruct-GGUF-llamafile/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.llamafile"
		filename = "qwen2.5-coder-1.5b-instruct-q4_k_m"
	case "llama-3.2-1b-instruct", "llama", "llama1b":
		url = "https://huggingface.co/Mozilla/Llama-3.2-1B-Instruct-llamafile/resolve/main/Llama-3.2-1B-Instruct.Q6_K.llamafile"
		filename = "Llama-3.2-1B-Instruct.Q6_K"
	default:
		return fmt.Errorf("unknown llamafile model: %s (available options: qwen2.5-coder-1.5b-instruct, llama-3.2-1b-instruct)", modelName)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	modelsDir := filepath.Join(home, ".local", "share", "promptyly", "models")
	_ = os.MkdirAll(modelsDir, 0755)

	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	modelPath := filepath.Join(modelsDir, filename+ext)

	if _, err := os.Stat(modelPath); err == nil {
		fmt.Printf("✓ Llamafile model is already downloaded at: %s\n", modelPath)
		return nil
	}

	// Try to use locally hosted llamafile from sharing server if configured and available
	sourceText := "from Hugging Face"
	if cfg, err := config.LoadConfig(); err == nil && cfg.SharingServerURL != "" {
		checkURL := fmt.Sprintf("%s/binaries/%s.llamafile", strings.TrimSuffix(cfg.SharingServerURL, "/"), filename)
		req, err := http.NewRequest("HEAD", checkURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", "Mozilla/5.0")
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					url = checkURL
					sourceText = "directly from your sharing server (local cache)"
				}
			}
		}
	}

	fmt.Printf("\n📥 Downloading %s llamafile (~1.2GB) %s...\n", modelName, sourceText)
	fmt.Println("This may take several minutes depending on your internet connection.")
	fmt.Printf("🔗 URL: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code from Hugging Face: %d", resp.StatusCode)
	}

	out, err := os.OpenFile(modelPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 32*1024)
	lastPercent := -1

	for {
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := out.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				return ew
			}
		}
		if er != nil {
			if er == io.EOF {
				break
			}
			return er
		}

		if total > 0 {
			percent := int(float64(written) / float64(total) * 100)
			if percent%10 == 0 && percent != lastPercent {
				fmt.Printf("... %d%% downloaded (%s/%s)\n", percent, formatBytes(written), formatBytes(total))
				lastPercent = percent
			}
		}
	}

	if err := os.Chmod(modelPath, 0755); err != nil {
		fmt.Printf("⚠️ Warning: could not set executable permissions: %v\n", err)
	}

	fmt.Println("✅ Download complete!")
	return nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

