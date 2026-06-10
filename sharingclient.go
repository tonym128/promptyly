package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"promptyly/app"
	"promptyly/config"
	"strings"
	"time"
)

func promptInput(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func PublishApp(cfg *config.Config, appName string) error {
	appDir, ok := cfg.Apps[appName]
	if !ok {
		return fmt.Errorf("app '%s' not found locally in your registry", appName)
	}

	// Double check directory exists
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		return fmt.Errorf("app directory '%s' does not exist", appDir)
	}

	// Resolve prompt text
	promptText := ""
	for pr, name := range cfg.Prompts {
		if name == appName {
			promptText = pr
			break
		}
	}
	if promptText == "" {
		promptText = promptInput("Enter the prompt used to create this app: ")
		if promptText == "" {
			return fmt.Errorf("prompt is required to publish")
		}
	}

	description := promptInput("Enter an optional description: ")

	serverURL := cfg.SharingServerURL
	if serverURL == "" {
		serverURL = "http://localhost:6072"
	}

	token := cfg.SharingToken
	if token == "" {
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
				return fmt.Errorf("login failed: %v", err)
			}
			cfg.SharingToken = token
			_ = config.SaveConfig(cfg)
			fmt.Println("✅ Sign in successful and token saved!")
		case "2":
			username := promptInput("Username: ")
			password := promptInput("Password (min 6 chars): ")
			token, err = registerRequest(serverURL, username, password)
			if err != nil {
				return fmt.Errorf("registration failed: %v", err)
			}
			cfg.SharingToken = token
			_ = config.SaveConfig(cfg)
			fmt.Println("✅ Account created and token saved!")
		case "3":
			token = promptInput("Paste sharing token: ")
			if token == "" {
				return fmt.Errorf("token cannot be empty")
			}
			cfg.SharingToken = token
			_ = config.SaveConfig(cfg)
			fmt.Println("✅ Token saved!")
		default:
			return fmt.Errorf("invalid option selected")
		}
	}

	// Package application as ZIP file
	tempZip := filepath.Join(os.TempDir(), fmt.Sprintf("promptyly-pub-%s.zip", appName))
	defer os.Remove(tempZip)

	fmt.Printf("\nPackaging application files from '%s'...\n", appDir)
	if err := app.ExportApp(cfg, appName, tempZip); err != nil {
		return fmt.Errorf("failed to package app: %v", err)
	}

	// Upload zip to server
	fmt.Printf("Publishing '%s' to %s...\n", appName, serverURL)
	err := uploadRequest(serverURL, token, tempZip, appName, promptText, description)
	if err != nil {
		return fmt.Errorf("upload failed: %v", err)
	}

	return nil
}

func loginRequest(serverURL, username, password string) (string, error) {
	u := fmt.Sprintf("%s/api/auth/login", strings.TrimSuffix(serverURL, "/"))
	bodyMap := map[string]string{"username": username, "password": password}
	bodyBytes, _ := json.Marshal(bodyMap)

	resp, err := http.Post(u, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Success bool   `json:"success"`
		Token   string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Token, nil
}

func registerRequest(serverURL, username, password string) (string, error) {
	u := fmt.Sprintf("%s/api/auth/register", strings.TrimSuffix(serverURL, "/"))
	bodyMap := map[string]string{"username": username, "password": password}
	bodyBytes, _ := json.Marshal(bodyMap)

	resp, err := http.Post(u, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Success bool   `json:"success"`
		Token   string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Token, nil
}

func uploadRequest(serverURL, token, zipPath, name, prompt, description string) error {
	file, err := os.Open(zipPath)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(zipPath))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}

	_ = writer.WriteField("name", name)
	_ = writer.WriteField("prompt", prompt)
	_ = writer.WriteField("description", description)
	_ = writer.Close()

	u := fmt.Sprintf("%s/api/apps/upload", strings.TrimSuffix(serverURL, "/"))
	req, err := http.NewRequest("POST", u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Success bool   `json:"success"`
		AppID   string `json:"app_id"`
		URL     string `json:"url"`
	}
	if err := json.Unmarshal(respBody, &res); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	cleanURL := strings.TrimSuffix(serverURL, "/") + res.URL
	detailURL := fmt.Sprintf("%s/app/%s", strings.TrimSuffix(serverURL, "/"), res.AppID)
	fmt.Printf("\n==================================================\n")
	fmt.Printf("✅ Application successfully published to the registry!\n")
	fmt.Printf("👉 Live URL: %s\n", cleanURL)
	fmt.Printf("👉 Detail Page: %s\n", detailURL)
	fmt.Printf("==================================================\n\n")

	return nil
}

func SearchApps(cfg *config.Config, query string) error {
	serverURL := cfg.SharingServerURL
	if serverURL == "" {
		serverURL = "http://localhost:6072"
	}

	u := fmt.Sprintf("%s/api/apps/search?q=%s", strings.TrimSuffix(serverURL, "/"), url.QueryEscape(query))
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(respBody))
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
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return err
	}

	if len(list) == 0 {
		fmt.Println("No matching applications found on the sharing server.")
		return nil
	}

	fmt.Printf("\nSearch Results from %s:\n", serverURL)
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

	return nil
}

func DownloadApp(cfg *config.Config, appID string) error {
	serverURL := cfg.SharingServerURL
	if serverURL == "" {
		serverURL = "http://localhost:6072"
	}

	u := fmt.Sprintf("%s/api/apps/download/%s", strings.TrimSuffix(serverURL, "/"), appID)
	fmt.Printf("Downloading app '%s' from %s...\n", appID, serverURL)

	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	tempZip := filepath.Join(os.TempDir(), fmt.Sprintf("promptyly-dl-%s.zip", appID))
	out, err := os.Create(tempZip)
	if err != nil {
		return err
	}
	defer os.Remove(tempZip)

	_, err = io.Copy(out, resp.Body)
	_ = out.Close()
	if err != nil {
		return err
	}

	fmt.Println("Importing application locally...")
	importedName, err := app.ImportApp(cfg, tempZip)
	if err != nil {
		return fmt.Errorf("failed to import: %v", err)
	}

	fmt.Printf("\n✅ App '%s' successfully downloaded and registered!\n", importedName)
	fmt.Printf("👉 Run it locally: promptyly run %s\n\n", importedName)
	return nil
}
