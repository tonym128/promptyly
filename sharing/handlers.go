package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Server struct {
	store   *Store
	dataDir string
}

func NewServer(store *Store, dataDir string) *Server {
	return &Server{
		store:   store,
		dataDir: dataDir,
	}
}

// getLoggedInUser parses session token from request cookies.
func (s *Server) getLoggedInUser(r *http.Request) *User {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return nil
	}
	user, exists := s.store.GetUserByToken(cookie.Value)
	if !exists {
		return nil
	}
	return user
}

// getAPIUser parses token from authorization headers or session cookies.
func (s *Server) getAPIUser(r *http.Request) *User {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if user, exists := s.store.GetUserByToken(token); exists {
			return user
		}
	}

	token := r.Header.Get("X-Promptyly-Token")
	if token != "" {
		if user, exists := s.store.GetUserByToken(token); exists {
			return user
		}
	}

	return s.getLoggedInUser(r)
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Web UI routes
	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/register", s.handleRegister)
	mux.HandleFunc("/logout", s.handleLogout)
	mux.HandleFunc("/upload", s.handleUpload)
	mux.HandleFunc("/profile", s.handleProfile)
	mux.HandleFunc("/app/", s.handleAppDetail)

	// REST API routes
	mux.HandleFunc("/api/auth/register", s.apiRegister)
	mux.HandleFunc("/api/auth/login", s.apiLogin)
	mux.HandleFunc("/api/apps/list", s.apiListApps)
	mux.HandleFunc("/api/apps/search", s.apiSearchApps)
	mux.HandleFunc("/api/apps/upload", s.apiUploadApp)
	mux.HandleFunc("/api/apps/download/", s.apiDownloadApp)

	// App static website serving
	mux.HandleFunc("/apps/", s.handleServeApp)
}

// handleHome renders the gallery page.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	user := s.getLoggedInUser(r)
	query := r.URL.Query().Get("q")
	var apps []*App

	if query != "" {
		apps = s.store.SearchApps(query)
	} else {
		apps = s.store.ListApps()
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(RenderHome(apps, query, user)))
}

// handleProfile renders the developer profile page.
func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	user := s.getLoggedInUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(RenderProfile(user)))
}

// handleLogin renders or processes logins.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(RenderLogin("", nil)))
		return
	}

	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		token, err := s.store.LoginUser(username, password)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(RenderLogin(err.Error(), nil)))
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Expires:  time.Now().Add(24 * 7 * time.Hour), // 1 week
			Path:     "/",
			HttpOnly: true,
		})

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// handleRegister processes registration.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(RenderRegister("", nil)))
		return
	}

	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		_, err := s.store.RegisterUser(username, password)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(RenderRegister(err.Error(), nil)))
			return
		}

		// Log them in immediately
		token, _ := s.store.LoginUser(username, password)
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Expires:  time.Now().Add(24 * 7 * time.Hour),
			Path:     "/",
			HttpOnly: true,
		})

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// handleLogout clears session.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Expires:  time.Unix(0, 0),
		Path:     "/",
		HttpOnly: true,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleUpload handles app upload page.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	user := s.getLoggedInUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(RenderUpload("", user)))
		return
	}

	if r.Method == "POST" {
		// Handle multipart form upload
		if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB limit
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(RenderUpload("File size exceeds 10MB limit", user)))
			return
		}

		file, handler, err := r.FormFile("file")
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(RenderUpload("Failed to parse file upload: "+err.Error(), user)))
			return
		}
		defer file.Close()

		name := r.FormValue("name")
		prompt := r.FormValue("prompt")
		description := r.FormValue("description")

		if strings.TrimSpace(prompt) == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(RenderUpload("Prompt is required", user)))
			return
		}

		// Save ZIP file to data/zips
		tempZipName := fmt.Sprintf("%d-%s", time.Now().UnixNano(), handler.Filename)
		zipPath := filepath.Join(s.dataDir, "zips", tempZipName)

		if err := os.MkdirAll(filepath.Dir(zipPath), 0755); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(RenderUpload("Failed to create zip folder: "+err.Error(), user)))
			return
		}

		out, err := os.Create(zipPath)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(RenderUpload("Failed to save zip file: "+err.Error(), user)))
			return
		}
		defer out.Close()

		if _, err = io.Copy(out, file); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(RenderUpload("Failed to write zip file: "+err.Error(), user)))
			return
		}

		// Register app metadata
		app, err := s.store.AddApp(user.ID, user.Username, name, prompt, description, tempZipName)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(RenderUpload("Database error: "+err.Error(), user)))
			return
		}

		// Extract ZIP file to data/apps/<app_id>
		destDir := filepath.Join(s.dataDir, "apps", app.ID)
		if err := extractZip(zipPath, destDir); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(RenderUpload("ZIP extraction failed: "+err.Error(), user)))
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/app/%s", app.ID), http.StatusSeeOther)
	}
}

// handleAppDetail renders app specifications.
func (s *Server) handleAppDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/app/")
	id = strings.Split(id, "/")[0] // Extract clean ID

	app, exists := s.store.GetApp(id)
	if !exists {
		http.NotFound(w, r)
		return
	}

	s.store.IncrementViews(id)

	user := s.getLoggedInUser(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(RenderAppDetail(app, user, "")))
}

// apiRegister endpoint
func (s *Server) apiRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := s.store.RegisterUser(req.Username, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"token":    user.Token,
		"username": user.Username,
	})
}

// apiLogin endpoint
func (s *Server) apiLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	token, err := s.store.LoginUser(req.Username, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"token":   token,
	})
}

// apiListApps lists registered apps.
func (s *Server) apiListApps(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apps := s.store.ListApps()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(apps)
}

// apiSearchApps searches registered apps.
func (s *Server) apiSearchApps(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	apps := s.store.SearchApps(query)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(apps)
}

// apiUploadApp processes machine integrations and client uploads.
func (s *Server) apiUploadApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := s.getAPIUser(r)
	if user == nil {
		http.Error(w, "Unauthorized: valid API token or login session required", http.StatusUnauthorized)
		return
	}

	// Limit to 10MB
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File exceeds 10MB limit", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read file parameter: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	name := r.FormValue("name")
	prompt := r.FormValue("prompt")
	description := r.FormValue("description")

	if strings.TrimSpace(prompt) == "" {
		http.Error(w, "Prompt is required", http.StatusBadRequest)
		return
	}

	// Save ZIP file
	tempZipName := fmt.Sprintf("%d-%s", time.Now().UnixNano(), handler.Filename)
	zipPath := filepath.Join(s.dataDir, "zips", tempZipName)

	if err := os.MkdirAll(filepath.Dir(zipPath), 0755); err != nil {
		http.Error(w, "Internal server error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out, err := os.Create(zipPath)
	if err != nil {
		http.Error(w, "Internal server error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer out.Close()

	if _, err = io.Copy(out, file); err != nil {
		http.Error(w, "Internal server error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Register app metadata
	app, err := s.store.AddApp(user.ID, user.Username, name, prompt, description, tempZipName)
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Extract ZIP file to apps directory
	destDir := filepath.Join(s.dataDir, "apps", app.ID)
	if err := extractZip(zipPath, destDir); err != nil {
		http.Error(w, "ZIP extraction failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"app_id":  app.ID,
		"url":     fmt.Sprintf("/apps/%s/", app.ID),
	})
}

// apiDownloadApp serves the zip file.
func (s *Server) apiDownloadApp(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/apps/download/")
	id = strings.Split(id, "/")[0] // Extract clean ID

	app, exists := s.store.GetApp(id)
	if !exists {
		http.NotFound(w, r)
		return
	}

	zipPath := filepath.Join(s.dataDir, "zips", app.ZipName)
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		http.Error(w, "ZIP source archive not found", http.StatusNotFound)
		return
	}

	s.store.IncrementDownloads(id)

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", app.ID))
	w.Header().Set("Content-Type", "application/zip")
	http.ServeFile(w, r, zipPath)
}

// handleServeApp serves static pages & the persistence API of hosted apps.
func (s *Server) handleServeApp(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/apps/") {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(strings.TrimPrefix(path, "/apps/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "App ID not specified", http.StatusBadRequest)
		return
	}
	appID := parts[0]

	// Check if app exists
	_, exists := s.store.GetApp(appID)
	if !exists {
		http.Error(w, fmt.Sprintf("App '%s' not found", appID), http.StatusNotFound)
		return
	}

	// Redirect if trailing slash is missing from app root directory
	if len(parts) == 1 && !strings.HasSuffix(path, "/") {
		http.Redirect(w, r, path+"/", http.StatusMovedPermanently)
		return
	}

	relPath := "/" + strings.Join(parts[1:], "/")
	appDir := filepath.Join(s.dataDir, "apps", appID)

	// Intercept dynamic persistence DB endpoint
	if relPath == "/_promptyly/api/db" || relPath == "/_promptyly/api/db/" {
		handleHostedAppDb(w, r, appID, s.dataDir)
		return
	}

	fileServerPath := filepath.Join(appDir, filepath.Clean(relPath))
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	http.ServeFile(w, r, fileServerPath)
}

// handleHostedAppDb processes dynamic relative DB storage for hosted apps.
func handleHostedAppDb(w http.ResponseWriter, r *http.Request, appID string, dataDir string) {
	dbPath := filepath.Join(dataDir, "apps", appID, ".promptyly", "db.json")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == "GET" {
		data, err := os.ReadFile(dbPath)
		if err != nil {
			if os.IsNotExist(err) {
				_, _ = w.Write([]byte("{}"))
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)
		return
	}

	if r.Method == "POST" {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var js json.RawMessage
		if err := json.Unmarshal(data, &js); err != nil {
			http.Error(w, "Invalid JSON format", http.StatusBadRequest)
			return
		}

		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(dbPath, data, 0644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, _ = w.Write([]byte(`{"status":"success"}`))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// extractZip unpacks file records to the destination path.
func extractZip(zipPath string, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		cleanedPath := filepath.Clean(file.Name)
		if strings.HasPrefix(cleanedPath, "..") || strings.HasPrefix(cleanedPath, "/") {
			continue // Skip path traversals
		}

		path := filepath.Join(destDir, cleanedPath)
		if file.FileInfo().IsDir() {
			_ = os.MkdirAll(path, file.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		rc, err := file.Open()
		if err != nil {
			_ = outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		_ = rc.Close()
		_ = outFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
