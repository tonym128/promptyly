package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"promptyly/agent"
	"promptyly/config"
	"sort"
	"strings"
	"sync"
	"time"
)

type Server struct {
	store        *Store
	dataDir      string
	llamafileCmd *exec.Cmd
	llamafileMu  sync.Mutex
}

func NewServer(store *Store, dataDir string) *Server {
	return &Server{
		store:   store,
		dataDir: dataDir,
	}
}

func (s *Server) StartLlamafile() {
	s.llamafileMu.Lock()
	defer s.llamafileMu.Unlock()

	if s.llamafileCmd != nil && s.llamafileCmd.Process != nil {
		return // Already running
	}

	modelPath := filepath.Join(s.dataDir, "binaries", "qwen2.5-coder-1.5b-instruct-q4_k_m.llamafile")
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		fmt.Printf("ℹ️ Llamafile model not found at %s. Cannot start LLM server.\n", modelPath)
		return
	}

	_ = os.Chmod(modelPath, 0755)

	fmt.Printf("🤖 Starting Llamafile LLM Server on port 6080...\n")
	cmd := exec.Command("sh", modelPath, "--server", "--port", "6080", "--host", "127.0.0.1", "-c", "2048", "--nobrowser")
	
	logPath := filepath.Join(s.dataDir, "llamafile.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	err = cmd.Start()
	if err != nil {
		fmt.Printf("❌ Failed to start Llamafile LLM Server: %v\n", err)
		return
	}

	s.llamafileCmd = cmd
	fmt.Printf("🤖 Llamafile LLM Server started on http://127.0.0.1:6080 (pid: %d). Logs: %s\n", cmd.Process.Pid, logPath)

	go func() {
		_ = cmd.Wait()
		s.llamafileMu.Lock()
		if s.llamafileCmd == cmd {
			s.llamafileCmd = nil
		}
		s.llamafileMu.Unlock()
		fmt.Println("🤖 Llamafile LLM Server process exited.")
	}()
}

func (s *Server) StopLlamafile() {
	s.llamafileMu.Lock()
	defer s.llamafileMu.Unlock()

	if s.llamafileCmd == nil || s.llamafileCmd.Process == nil {
		return
	}

	fmt.Printf("🤖 Stopping Llamafile LLM Server (pid: %d)...\n", s.llamafileCmd.Process.Pid)
	_ = s.llamafileCmd.Process.Kill()
	s.llamafileCmd = nil
}

func (s *Server) trackEvent(r *http.Request, category, action, label string) {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			ip = ip[:idx]
		}
	} else {
		if idx := strings.Index(ip, ","); idx != -1 {
			ip = strings.TrimSpace(ip[:idx])
		}
	}
	ua := r.UserAgent()
	s.store.RecordEvent(category, action, label, ip, ua)
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
	if !user.IsApproved && !user.IsAdmin {
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
			if user.IsApproved || user.IsAdmin {
				return user
			}
		}
	}

	token := r.Header.Get("X-Promptyly-Token")
	if token != "" {
		if user, exists := s.store.GetUserByToken(token); exists {
			if user.IsApproved || user.IsAdmin {
				return user
			}
		}
	}

	return s.getLoggedInUser(r)
}

func (s *Server) requireLoginMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/login" || path == "/register" || path == "/logout" ||
			path == "/install.sh" || path == "/install.ps1" ||
			path == "/api/version/check" ||
			strings.HasPrefix(path, "/binaries/") {
			next(w, r)
			return
		}

		requireLogin := s.store.GetConfig().RequireLoginToView
		if requireLogin {
			isAPI := strings.HasPrefix(path, "/api/")
			var user *User
			if isAPI {
				user = s.getAPIUser(r)
			} else {
				user = s.getLoggedInUser(r)
			}

			if user == nil {
				if isAPI {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   "Authentication required",
					})
				} else {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
				}
				return
			}
		}

		next(w, r)
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	wrap := s.requireLoginMiddleware

	// Web UI routes
	mux.HandleFunc("/", wrap(s.handleHome))
	mux.HandleFunc("/registry", wrap(s.handleRegistry))
	mux.HandleFunc("/login", wrap(s.handleLogin))
	mux.HandleFunc("/register", wrap(s.handleRegister))
	mux.HandleFunc("/logout", wrap(s.handleLogout))
	mux.HandleFunc("/upload", wrap(s.handleUpload))
	mux.HandleFunc("/profile", wrap(s.handleProfile))
	mux.HandleFunc("/extension", wrap(s.handleExtensionHowTo))
	mux.HandleFunc("/app/", wrap(s.handleAppDetail))
	mux.HandleFunc("/admin", wrap(s.handleAdminPanel))

	// REST API routes
	mux.HandleFunc("/api/auth/register", wrap(s.apiRegister))
	mux.HandleFunc("/api/auth/login", wrap(s.apiLogin))
	mux.HandleFunc("/api/auth/me", wrap(s.apiMe))
	mux.HandleFunc("/api/auth/update-password", wrap(s.apiUpdatePassword))
	mux.HandleFunc("/api/apps/list", wrap(s.apiListApps))
	mux.HandleFunc("/api/apps/search", wrap(s.apiSearchApps))
	mux.HandleFunc("/api/apps/upload", wrap(s.apiUploadApp))
	mux.HandleFunc("/api/apps/download/", wrap(s.apiDownloadApp))
	mux.HandleFunc("/api/apps/delete/", wrap(s.apiDeleteApp))
	mux.HandleFunc("/api/apps/create", wrap(s.apiCreateApp))
	
	// LLM proxy route
	targetUrl, _ := url.Parse("http://127.0.0.1:6080")
	proxy := httputil.NewSingleHostReverseProxy(targetUrl)
	mux.HandleFunc("/api/llm/", wrap(func(w http.ResponseWriter, r *http.Request) {
		user := s.getAPIUser(r)
		if user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Authentication required (active session cookie or Bearer API token)",
			})
			return
		}

		s.trackEvent(r, "llm", "proxy", "completion")
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api/llm")
		proxy.ServeHTTP(w, r)
	}))

	mux.HandleFunc("/api/version/check", wrap(s.apiVersionCheck))

	// Admin API routes
	mux.HandleFunc("/api/admin/approve", wrap(s.apiAdminApproveUser))
	mux.HandleFunc("/api/admin/reject", wrap(s.apiAdminRejectUser))
	mux.HandleFunc("/api/admin/config", wrap(s.apiAdminUpdateConfig))

	// App static website serving
	mux.HandleFunc("/apps/", wrap(s.handleServeApp))

	// Installer and Uninstaller script routes
	mux.HandleFunc("/install.sh", wrap(s.handleInstallSh))
	mux.HandleFunc("/install.ps1", wrap(s.handleInstallPs1))
	mux.HandleFunc("/uninstall.sh", wrap(s.handleUninstallSh))
	mux.HandleFunc("/uninstall.ps1", wrap(s.handleUninstallPs1))

	// Binary assets serving
	binariesDir := filepath.Join(s.dataDir, "binaries")
	binaryHandler := http.StripPrefix("/binaries/", http.FileServer(http.Dir(binariesDir)))
	mux.HandleFunc("/binaries/", wrap(func(w http.ResponseWriter, r *http.Request) {
		filename := strings.TrimPrefix(r.URL.Path, "/binaries/")
		if filename != "" {
			s.trackEvent(r, "link", "download", "binary:"+filename)
		}
		binaryHandler.ServeHTTP(w, r)
	}))
}

// handleHome renders the landing page.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	s.trackEvent(r, "page", "view", "home")
	user := s.getLoggedInUser(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(RenderLandingPage(user)))
}

// handleRegistry renders the gallery page.
func (s *Server) handleRegistry(w http.ResponseWriter, r *http.Request) {
	user := s.getLoggedInUser(r)
	query := r.URL.Query().Get("q")
	var apps []*App

	s.trackEvent(r, "page", "view", "registry")
	if query != "" {
		apps = s.store.SearchApps(query)
	} else {
		apps = s.store.ListApps()
	}

	sortBy := r.URL.Query().Get("sort")
	order := r.URL.Query().Get("order")
	if sortBy == "" {
		sortBy = "upload_date"
	}
	if order == "" {
		if sortBy == "upload_date" || sortBy == "views" {
			order = "desc"
		} else {
			order = "asc"
		}
	}
	sortApps(apps, sortBy, order)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(RenderHome(apps, query, sortBy, order, user)))
}

// handleProfile renders the developer profile page.
func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	user := s.getLoggedInUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	s.trackEvent(r, "page", "view", "profile")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(RenderProfile(user)))
}

// handleExtensionHowTo renders the browser extension installation guide.
func (s *Server) handleExtensionHowTo(w http.ResponseWriter, r *http.Request) {
	user := s.getLoggedInUser(r)
	s.trackEvent(r, "page", "view", "extension_howto")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(RenderExtensionHowTo(user)))
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
	allowSelfReg := s.store.GetConfig().AllowSelfRegistration
	if !allowSelfReg {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<h3>Registration is disabled on this server.</h3><p><a href='/login'>Back to login</a></p>"))
		return
	}

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

		// Log them in immediately if approved
		token, loginErr := s.store.LoginUser(username, password)
		if loginErr != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if loginErr.Error() == "account pending admin approval" {
				_, _ = w.Write([]byte(RenderLogin("Account registered successfully! Pending admin approval.", nil)))
			} else {
				_, _ = w.Write([]byte(RenderLogin("Registration successful. Please log in.", nil)))
			}
			return
		}

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

	s.trackEvent(r, "app", "view_details", id)
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
	allowSelfReg := s.store.GetConfig().AllowSelfRegistration
	if !allowSelfReg {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Self-registration is disabled on this server",
		})
		return
	}

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

	// Try to login to obtain token
	token, loginErr := s.store.LoginUser(req.Username, req.Password)
	if loginErr != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"pending": true,
			"message": "Registration successful, pending admin approval",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"token":    token,
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
	sortBy := r.URL.Query().Get("sort")
	order := r.URL.Query().Get("order")
	if sortBy != "" {
		if order == "" {
			if sortBy == "upload_date" || sortBy == "views" {
				order = "desc"
			} else {
				order = "asc"
			}
		}
		sortApps(apps, sortBy, order)
	}

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
	s.trackEvent(r, "url", "search", query)
	apps := s.store.SearchApps(query)
	sortBy := r.URL.Query().Get("sort")
	order := r.URL.Query().Get("order")
	if sortBy != "" {
		if order == "" {
			if sortBy == "upload_date" || sortBy == "views" {
				order = "desc"
			} else {
				order = "asc"
			}
		}
		sortApps(apps, sortBy, order)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(apps)
}

// apiMe returns the currently authenticated user's profile info.
func (s *Server) apiMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := s.getAPIUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"username": user.Username,
	})
}

func (s *Server) apiUpdatePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := s.getLoggedInUser(r)
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Unauthorized"})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	if err := s.store.UpdatePassword(user.Username, req.CurrentPassword, req.NewPassword); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
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

	s.trackEvent(r, "upload", "publish", app.ID)

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
	s.trackEvent(r, "app", "download", id)

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

	if relPath == "/" || relPath == "/index.html" {
		s.trackEvent(r, "app", "view", appID)
	}

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

// apiDeleteApp deletes the app from the sharing registry.
func (s *Server) apiDeleteApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" && r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := s.getAPIUser(r)
	if user == nil {
		http.Error(w, "Unauthorized: valid API token or login session required", http.StatusUnauthorized)
		return
	}

	var req struct {
		AppID string `json:"app_id"`
	}

	// Try reading from JSON body
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	// Fallback: check query parameter
	if req.AppID == "" {
		req.AppID = r.URL.Query().Get("app_id")
	}

	// Fallback: check URL path
	if req.AppID == "" {
		req.AppID = strings.TrimPrefix(r.URL.Path, "/api/apps/delete/")
		req.AppID = strings.Split(req.AppID, "/")[0] // Extract clean ID
	}

	if req.AppID == "" {
		http.Error(w, "app_id is required", http.StatusBadRequest)
		return
	}

	app, exists := s.store.GetApp(req.AppID)
	if !exists {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if app.UserID != user.ID {
		http.Error(w, "Forbidden: you do not own this application", http.StatusForbidden)
		return
	}

	// Delete ZIP file
	if app.ZipName != "" {
		zipPath := filepath.Join(s.dataDir, "zips", app.ZipName)
		_ = os.Remove(zipPath)
	}

	// Delete extracted folder
	destDir := filepath.Join(s.dataDir, "apps", app.ID)
	_ = os.RemoveAll(destDir)

	// Remove from database
	if err := s.store.DeleteApp(app.ID); err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Application deleted successfully from registry",
	})
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

func (s *Server) handleInstallSh(w http.ResponseWriter, r *http.Request) {
	s.trackEvent(r, "link", "download", "install.sh")
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host

	// Check if local llamafile is hosted on this registry server
	localLlamafileUrl := "https://huggingface.co/Bojun-Feng/Qwen2.5-Coder-1.5B-Instruct-GGUF-llamafile/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf"
	binariesDir := filepath.Join(s.dataDir, "binaries")
	localPath := filepath.Join(binariesDir, "qwen2.5-coder-1.5b-instruct-q4_k_m.llamafile")
	isLocal := false
	if _, err := os.Stat(localPath); err == nil {
		localLlamafileUrl = fmt.Sprintf("%s://%s/binaries/qwen2.5-coder-1.5b-instruct-q4_k_m.llamafile", scheme, host)
		isLocal = true
	}

	sourceText := "from Hugging Face"
	if isLocal {
		sourceText = "directly from our sharing server (local cache)"
	}

	script := fmt.Sprintf(`#!/bin/sh
set -e

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    armv7l|armv8l|arm) ARCH="arm" ;;
    *) echo "❌ Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    darwin) OS_NAME="darwin" ;;
    linux)
        if [ -d "/data/data/com.termux" ] || [ "$(uname -o 2>/dev/null)" = "Android" ]; then
            OS_NAME="android"
        else
            OS_NAME="linux"
        fi
        ;;
    *) echo "❌ Unsupported OS: $OS"; exit 1 ;;
esac

if [ "${OS_NAME}" = "android" ] && [ "${ARCH}" = "arm" ]; then
    echo "❌ Android 32-bit is not supported. Only Android ARM64 is supported."
    exit 1
fi

BINARY_NAME="promptyly-${OS_NAME}-${ARCH}"
DOWNLOAD_URL="%s://%s/binaries/${BINARY_NAME}"

INSTALL_DIR="${HOME}/.local/bin"
if [ "${OS_NAME}" = "android" ] && [ -n "${PREFIX}" ]; then
    INSTALL_DIR="${PREFIX}/bin"
fi

mkdir -p "${INSTALL_DIR}"
INSTALL_PATH="${INSTALL_DIR}/promptyly"

echo "📥 Downloading Promptyly CLI (${OS_NAME}/${ARCH})..."
echo "🔗 URL: ${DOWNLOAD_URL}"

# Stop running daemon instances to release file locks
if command -v systemctl >/dev/null 2>&1; then
    systemctl --user stop promptyly.service 2>/dev/null || true
fi
if [ "${OS_NAME}" = "darwin" ]; then
    launchctl unload "${HOME}/Library/LaunchAgents/com.promptyly.daemon.plist" 2>/dev/null || true
fi
if command -v pkill >/dev/null 2>&1; then
    pkill -f "promptyly serve" || true
else
    PID=$(ps aux | grep "[p]romptyly serve" | grep -v grep | awk '{print $2}')
    if [ -n "$PID" ]; then
        kill "$PID" || true
    fi
fi

# Remove existing binary first to avoid "Text file busy" write errors
rm -f "${INSTALL_PATH}"

if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${DOWNLOAD_URL}" -o "${INSTALL_PATH}"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "${INSTALL_PATH}" "${DOWNLOAD_URL}"
else
    echo "❌ Neither curl nor wget found. Please install one."
    exit 1
fi

chmod +x "${INSTALL_PATH}"

# Pre-configure CLI to point to this registry server
"${INSTALL_PATH}" config set sharing_server_url "%s://%s"

# Register URL protocol handler
echo "⚙️ Registering prompt:// URL scheme handler..."
if ! "${INSTALL_PATH}" register; then
    echo "⚠️ URL scheme registration failed. You can run 'promptyly register' manually later."
fi

# Setup systemd user service on Linux
if [ "${OS_NAME}" = "linux" ] && command -v systemctl >/dev/null 2>&1; then
    echo "⚙️ Setting up systemd user service for Promptyly daemon..."
    mkdir -p "${HOME}/.config/systemd/user"
    cat <<EOF > "${HOME}/.config/systemd/user/promptyly.service"
[Unit]
Description=Promptyly Developer Daemon
After=network.target

[Service]
ExecStart=${INSTALL_PATH} serve
Restart=on-failure
Environment=HOST=127.0.0.1

[Install]
WantedBy=default.target
EOF
    systemctl --user daemon-reload
    systemctl --user enable promptyly.service
    systemctl --user restart promptyly.service
    echo "✓ systemd user service enabled and started!"
fi

# Setup launchd agent on macOS
if [ "${OS_NAME}" = "darwin" ]; then
    echo "⚙️ Setting up launchd user agent for Promptyly daemon..."
    mkdir -p "${HOME}/Library/LaunchAgents"
    cat <<EOF > "${HOME}/Library/LaunchAgents/com.promptyly.daemon.plist"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.promptyly.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_PATH}</string>
        <string>serve</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOST</key>
        <string>127.0.0.1</string>
    </dict>
</dict>
</plist>
EOF
    launchctl unload "${HOME}/Library/LaunchAgents/com.promptyly.daemon.plist" 2>/dev/null || true
    launchctl load "${HOME}/Library/LaunchAgents/com.promptyly.daemon.plist"
    echo "✓ launchd user agent enabled and started!"
fi

echo ""
echo "--------------------------------------------------"
echo "🤖 LLM Provider Setup"
echo "--------------------------------------------------"
echo "Promptyly needs a local or remote LLM provider to function."
echo "Choose one of the following options:"
echo "1) Configure LLM via API Key or URL (Gemini, Claude, Ollama, OpenAI)"
echo "2) Install a local CPU coding model (Qwen2.5-Coder-1.5B via llamafile, ~1.2GB, runs on 4GB RAM)"
echo "3) Use this Remote Registry server's Llamafile LLM"
echo "4) Skip configuration (you can run 'promptyly config setup' later)"
echo ""
printf "Enter choice (1-4) [default: 4]: "
read CHOICE < /dev/tty
if [ -z "$CHOICE" ]; then
    CHOICE="4"
fi

if [ "$CHOICE" = "1" ]; then
    echo ""
    echo "Select LLM provider:"
    echo "1) Gemini (Recommended - Google)"
    echo "2) Claude (Anthropic)"
    echo "3) Ollama (Local LLM Server)"
    echo "4) OpenAI-compatible / LM Studio"
    printf "Choose option (1-4) [default: 1]: "
    read PROVIDER_CHOICE < /dev/tty
    
    PROVIDER="gemini"
    if [ "$PROVIDER_CHOICE" = "2" ]; then
        PROVIDER="claude"
    elif [ "$PROVIDER_CHOICE" = "3" ]; then
        PROVIDER="ollama"
    elif [ "$PROVIDER_CHOICE" = "4" ]; then
        PROVIDER="lmstudio"
    fi
    
    "${INSTALL_PATH}" config set default_provider "$PROVIDER"
    
    if [ "$PROVIDER" = "gemini" ]; then
        printf "Enter Gemini API Key: "
        read API_KEY < /dev/tty
        if [ -n "$API_KEY" ]; then
            "${INSTALL_PATH}" config set gemini_key "$API_KEY"
        fi
    elif [ "$PROVIDER" = "claude" ]; then
        printf "Enter Claude API Key: "
        read API_KEY < /dev/tty
        if [ -n "$API_KEY" ]; then
            "${INSTALL_PATH}" config set claude_key "$API_KEY"
        fi
    elif [ "$PROVIDER" = "ollama" ]; then
        printf "Enter Ollama Endpoint URL [default: http://localhost:11434]: "
        read OLLAMA_URL < /dev/tty
        if [ -n "$OLLAMA_URL" ]; then
            "${INSTALL_PATH}" config set ollama_url "$OLLAMA_URL"
        fi
        printf "Enter Ollama Model [default: llama3]: "
        read OLLAMA_MODEL < /dev/tty
        if [ -n "$OLLAMA_MODEL" ]; then
            "${INSTALL_PATH}" config set ollama_model "$OLLAMA_MODEL"
        fi
    elif [ "$PROVIDER" = "lmstudio" ]; then
        printf "Enter Endpoint URL [default: http://localhost:1234/v1]: "
        read LM_URL < /dev/tty
        if [ -n "$LM_URL" ]; then
            "${INSTALL_PATH}" config set lmstudio_url "$LM_URL"
        fi
        printf "Enter Model name [default: meta-llama-3-8b-instruct]: "
        read LM_MODEL < /dev/tty
        if [ -n "$LM_MODEL" ]; then
            "${INSTALL_PATH}" config set lmstudio_model "$LM_MODEL"
        fi
    fi
elif [ "$CHOICE" = "2" ]; then
    echo ""
    echo "📥 Downloading Qwen2.5-Coder-1.5B llamafile (~1.2GB) %s..."
    echo "This might take several minutes depending on your internet connection."
    
    MODELS_DIR="${HOME}/.local/share/promptyly/models"
    mkdir -p "${MODELS_DIR}"
    MODEL_PATH="${MODELS_DIR}/qwen2.5-coder-1.5b-instruct-q4_k_m.llamafile"
    
    DOWNLOAD_URL="%s"
    
    if command -v curl >/dev/null 2>&1; then
        curl -L -f -# "${DOWNLOAD_URL}" -o "${MODEL_PATH}"
    elif command -v wget >/dev/null 2>&1; then
        wget --show-progress -O "${MODEL_PATH}" "${DOWNLOAD_URL}"
    else
        echo "❌ Neither curl nor wget found. Cannot download llamafile."
        exit 1
    fi
    
    chmod +x "${MODEL_PATH}"
    
    # Configure CLI to use this llamafile
    "${INSTALL_PATH}" config set default_provider "lmstudio"
    "${INSTALL_PATH}" config set lmstudio_url "http://localhost:6073/v1"
    "${INSTALL_PATH}" config set lmstudio_model "qwen2.5-coder-1.5b-instruct"
    
    echo ""
    echo "✅ Qwen2.5-Coder-1.5B llamafile successfully installed to ${MODEL_PATH}"
    echo "🤖 Configure complete: default provider set to Local Llamafile at http://localhost:6073/v1"
    echo ""
    echo "And keep the terminal window open while using Promptyly."
elif [ "$CHOICE" = "3" ]; then
    echo ""
    echo "🤖 Configuring to use remote registry Llamafile..."
    "${INSTALL_PATH}" config set default_provider "registry"
    "${INSTALL_PATH}" config set sharing_server_url "%[1]s://%[2]s"
    printf "Enter your Registry API Token (Optional): "
    read API_KEY < /dev/tty
    if [ -n "$API_KEY" ]; then
        "${INSTALL_PATH}" config set sharing_token "$API_KEY"
    fi
fi

echo ""
echo "✅ Installed successfully to ${INSTALL_PATH}"
echo ""
if [ "${OS_NAME}" != "android" ] || [ -z "${PREFIX}" ]; then
    echo "⚙️  PATH configuration:"
    echo "   Ensure your PATH includes the installation directory:"
    echo "   export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo ""
fi

echo "--------------------------------------------------"
echo "🎉 Welcome to Promptyly!"
echo "--------------------------------------------------"
echo "Here are the things you can do now:"
echo "👉 Setup different models:"
echo "   Run 'promptyly config setup' to configure Gemini, Claude, Ollama, or OpenAI."
echo "👉 Run with llamafile:"
echo "   Select option 5 in 'promptyly config setup' to download the local CPU coding model."
echo "👉 Run promptly commands:"
echo "   Run 'promptyly create \"<prompt>\"' to generate a new app."
echo "   Run 'promptyly run <app-name>' to edit an app interactively."
echo "👉 Visit the local registry:"
echo "   Start the background daemon: 'promptyly serve' (if it's not already running)"
echo "   Then open your browser to: http://localhost:6071"
echo "👉 Setup Remote Registry in Promptyly:"
echo "   1. Log in to this registry server at: %[1]s://%[2]s"
echo "   2. Go to your Profile page, copy your API Token, and configure it:"
echo "      'promptyly config set sharing_token <your-token>'"
echo "   3. Point your CLI to this registry:"
echo "      'promptyly config set sharing_server_url %[1]s://%[2]s'"
echo "👉 Visit the remote registry:"
echo "   Open the sharing server at: %[1]s://%[2]s"
echo "   Or search registry via CLI: 'promptyly search \"<query>\"'"
echo "👉 Uninstall the program and service:"
echo "   Run 'promptyly uninstall' or run the uninstaller script:"
echo "   curl -fsSL %[1]s://%[2]s/uninstall.sh | bash"
echo "--------------------------------------------------"`, scheme, host, scheme, host, sourceText, localLlamafileUrl)

	w.Header().Set("Content-Type", "text/x-sh")
	_, _ = w.Write([]byte(script))
}

func (s *Server) handleInstallPs1(w http.ResponseWriter, r *http.Request) {
	s.trackEvent(r, "link", "download", "install.ps1")
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host

	// Check if local llamafile is hosted on this registry server
	localLlamafileUrl := "https://huggingface.co/Bojun-Feng/Qwen2.5-Coder-1.5B-Instruct-GGUF-llamafile/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf"
	binariesDir := filepath.Join(s.dataDir, "binaries")
	localPath := filepath.Join(binariesDir, "qwen2.5-coder-1.5b-instruct-q4_k_m.llamafile")
	isLocal := false
	if _, err := os.Stat(localPath); err == nil {
		localLlamafileUrl = fmt.Sprintf("%s://%s/binaries/qwen2.5-coder-1.5b-instruct-q4_k_m.llamafile", scheme, host)
		isLocal = true
	}

	sourceText := "from Hugging Face"
	if isLocal {
		sourceText = "directly from our sharing server (local cache)"
	}

	script := fmt.Sprintf(`$ErrorActionPreference = "Stop"

$rawArch = $env:PROCESSOR_ARCHITECTURE
$arch6432 = $env:PROCESSOR_ARCHITEW6432

if ($rawArch -eq "ARM64" -or $arch6432 -eq "ARM64") {
    $targetArch = "arm64"
} elseif ($rawArch -eq "AMD64" -or $arch6432 -eq "AMD64") {
    $targetArch = "amd64"
} else {
    Write-Error "❌ Unsupported architecture: $rawArch"
    exit 1
}

$installDir = Join-Path $HOME ".local\bin"
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
}

$installPath = Join-Path $installDir "promptyly.exe"
$downloadUrl = "%s://%s/binaries/promptyly-windows-$targetArch.exe"

Write-Host "📥 Downloading Promptyly CLI (windows/$targetArch)..." -ForegroundColor Cyan
Write-Host "🔗 URL: $downloadUrl" -ForegroundColor Gray

# Stop running daemon processes to release file locks on the executable
$process = Get-Process -Name "promptyly" -ErrorAction SilentlyContinue
if ($process) {
    Write-Host "🔌 Stopping running daemon..." -ForegroundColor Yellow
    Stop-Process -Name "promptyly" -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
}

Invoke-RestMethod -Uri $downloadUrl -OutFile $installPath

# Pre-configure CLI to point to this registry server
& $installPath config set sharing_server_url "%s://%s"

# Register URL protocol handler
Write-Host "⚙️ Registering prompt:// URL scheme handler..." -ForegroundColor Yellow
try {
    & $installPath register
} catch {
    Write-Warning "⚠️ URL scheme registration failed. You can run 'promptyly register' manually later."
}

Write-Host ""
Write-Host "--------------------------------------------------" -ForegroundColor Cyan
Write-Host "🤖 LLM Provider Setup" -ForegroundColor Cyan
Write-Host "--------------------------------------------------" -ForegroundColor Cyan
Write-Host "Promptyly needs a local or remote LLM provider to function."
Write-Host "Choose one of the following options:"
Write-Host "1) Configure LLM via API Key or URL (Gemini, Claude, Ollama, OpenAI)"
Write-Host "2) Install a local CPU coding model (Qwen2.5-Coder-1.5B via llamafile, ~1.2GB, runs on 4GB RAM)"
Write-Host "3) Use this Remote Registry server's Llamafile LLM"
Write-Host "4) Skip configuration (you can run 'promptyly config setup' later)"
Write-Host ""
$choice = Read-Host "Enter choice (1-4) [default: 4]"
if (-not $choice) { $choice = "4" }

if ($choice -eq "1") {
    Write-Host ""
    Write-Host "Select LLM provider:"
    Write-Host "1) Gemini (Recommended - Google)"
    Write-Host "2) Claude (Anthropic)"
    Write-Host "3) Ollama (Local LLM Server)"
    Write-Host "4) OpenAI-compatible / LM Studio"
    $providerChoice = Read-Host "Choose option (1-4) [default: 1]"
    if (-not $providerChoice) { $providerChoice = "1" }

    $provider = "gemini"
    if ($providerChoice -eq "2") {
        $provider = "claude"
    } elseif ($providerChoice -eq "3") {
        $provider = "ollama"
    } elseif ($providerChoice -eq "4") {
        $provider = "lmstudio"
    }

    & $installPath config set default_provider $provider

    if ($provider -eq "gemini") {
        $apiKey = Read-Host "Enter Gemini API Key"
        if ($apiKey) {
            & $installPath config set gemini_key $apiKey
        }
    } elseif ($provider -eq "claude") {
        $apiKey = Read-Host "Enter Claude API Key"
        if ($apiKey) {
            & $installPath config set claude_key $apiKey
        }
    } elseif ($provider -eq "ollama") {
        $ollamaUrl = Read-Host "Enter Ollama Endpoint URL [default: http://localhost:11434]"
        if ($ollamaUrl) {
            & $installPath config set ollama_url $ollamaUrl
        }
        $ollamaModel = Read-Host "Enter Ollama Model [default: llama3]"
        if ($ollamaModel) {
            & $installPath config set ollama_model $ollamaModel
        }
    } elseif ($provider -eq "lmstudio") {
        $lmUrl = Read-Host "Enter Endpoint URL [default: http://localhost:1234/v1]"
        if ($lmUrl) {
            & $installPath config set lmstudio_url $lmUrl
        }
        $lmModel = Read-Host "Enter Model name [default: meta-llama-3-8b-instruct]"
        if ($lmModel) {
            & $installPath config set lmstudio_model $lmModel
        }
    }
} elseif ($choice -eq "2") {
    Write-Host ""
    Write-Host "📥 Downloading Qwen2.5-Coder-1.5B llamafile (~1.2GB) %s..." -ForegroundColor Yellow
    Write-Host "This might take several minutes depending on your internet connection." -ForegroundColor Yellow

    $modelsDir = Join-Path $HOME ".local\share\promptyly\models"
    if (-not (Test-Path $modelsDir)) {
        New-Item -ItemType Directory -Force -Path $modelsDir | Out-Null
    }
    $modelPath = Join-Path $modelsDir "qwen2.5-coder-1.5b-instruct-q4_k_m.exe"
    $downloadUrl = "%s"

    Write-Host "🔗 URL: $downloadUrl" -ForegroundColor Gray
    Invoke-WebRequest -Uri $downloadUrl -OutFile $modelPath -UserAgent "Mozilla/5.0"

    # Configure CLI to use this llamafile
    & $installPath config set default_provider "lmstudio"
    & $installPath config set lmstudio_url "http://localhost:6073/v1"
    & $installPath config set lmstudio_model "qwen2.5-coder-1.5b-instruct"

    Write-Host ""
    Write-Host "✅ Qwen2.5-Coder-1.5B llamafile successfully installed to $modelPath" -ForegroundColor Green
    Write-Host "🤖 Configure complete: default provider set to Local Llamafile at http://localhost:6073/v1" -ForegroundColor Green
    Write-Host "And keep the terminal window open while using Promptyly."
} elseif ($choice -eq "3") {
    Write-Host ""
    Write-Host "🤖 Configuring to use remote registry Llamafile..." -ForegroundColor Yellow
    & $installPath config set default_provider "registry"
    & $installPath config set sharing_server_url "%s://%s"
    $apiKey = Read-Host "Enter your Registry API Token (Optional)"
    if ($apiKey) {
        & $installPath config set sharing_token $apiKey
    }
}

Write-Host ""
Write-Host "✅ Installed successfully to $installPath" -ForegroundColor Green

$userPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    $newUserPath = "$userPath;$installDir"
    [System.Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
    Write-Host "✏️ Added $installDir to User PATH environment variable." -ForegroundColor Yellow
    Write-Host "👉 Please restart your terminal/PowerShell for changes to take effect." -ForegroundColor Yellow
}

# Setup Scheduled Task on Windows for autostart
Write-Host "⚙️ Setting up Windows Scheduled Task for Promptyly daemon..." -ForegroundColor Yellow
try {
    $taskName = "PromptylyDaemon"
    $action = New-ScheduledTaskAction -Execute "$installPath" -Argument "serve"
    $trigger = New-ScheduledTaskTrigger -AtLogOn
    $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
    $principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive
    
    # Check if task already exists and unregister it
    Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue | Unregister-ScheduledTask -Confirm:$false
    
    Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal | Out-Null
    
    # Start the task now
    Start-ScheduledTask -TaskName $taskName
    Write-Host "✓ Windows Scheduled Task registered and started successfully!" -ForegroundColor Green
} catch {
    Write-Warning "⚠️ Failed to configure Scheduled Task: $_. You can run the daemon manually using: promptyly serve"
}

Write-Host ""
Write-Host "--------------------------------------------------" -ForegroundColor Cyan
Write-Host "🎉 Welcome to Promptyly!" -ForegroundColor Green
Write-Host "--------------------------------------------------" -ForegroundColor Cyan
Write-Host "Here are the things you can do now:"
Write-Host "👉 Setup different models:" -ForegroundColor Yellow
Write-Host "   Run 'promptyly config setup' to configure Gemini, Claude, Ollama, or OpenAI."
Write-Host "👉 Run with llamafile:" -ForegroundColor Yellow
Write-Host "   Select option 5 in 'promptyly config setup' to download the local CPU coding model."
Write-Host "👉 Run promptly commands:" -ForegroundColor Yellow
Write-Host "   Run 'promptyly create \"<prompt>\"' to generate a new app."
Write-Host "   Run 'promptyly run <app-name>' to edit an app interactively."
Write-Host "👉 Visit the local registry:" -ForegroundColor Yellow
Write-Host "   Start the background daemon: 'promptyly serve' (if it's not already running)"
Write-Host "   Then open your browser to: http://localhost:6071"
Write-Host "👉 Setup Remote Registry in Promptyly:" -ForegroundColor Yellow
Write-Host "   1. Log in to this registry server at: %[1]s://%[2]s"
Write-Host "   2. Go to your Profile page, copy your API Token, and configure it:"
Write-Host "      'promptyly config set sharing_token <your-token>'"
Write-Host "   3. Point your CLI to this registry:"
Write-Host "      'promptyly config set sharing_server_url %[1]s://%[2]s'"
Write-Host "👉 Visit the remote registry:" -ForegroundColor Yellow
Write-Host "   Open the sharing server at: %[1]s://%[2]s"
Write-Host "   Or search registry via CLI: 'promptyly search \"<query>\"'"
Write-Host "👉 Uninstall the program and service:" -ForegroundColor Yellow
Write-Host "   Run 'promptyly uninstall' or run the uninstaller script:"
Write-Host "   irm %[1]s://%[2]s/uninstall.ps1 | iex"
Write-Host "--------------------------------------------------" -ForegroundColor Cyan
`, scheme, host, scheme, host, sourceText, localLlamafileUrl)

	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(script))
}

func (s *Server) handleAdminPanel(w http.ResponseWriter, r *http.Request) {
	currentUser := s.getLoggedInUser(r)
	if currentUser == nil || !currentUser.IsAdmin {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	users := s.store.ListUsers()
	analytics := s.store.GetAnalytics()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(RenderAdminPanel(users, currentUser, s.store.GetConfig(), analytics)))
}

func (s *Server) apiAdminApproveUser(w http.ResponseWriter, r *http.Request) {
	currentUser := s.getLoggedInUser(r)
	if currentUser == nil || !currentUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.store.ApproveUser(req.Username); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func (s *Server) apiAdminRejectUser(w http.ResponseWriter, r *http.Request) {
	currentUser := s.getLoggedInUser(r)
	if currentUser == nil || !currentUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.store.RejectUser(req.Username); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func (s *Server) apiAdminUpdateConfig(w http.ResponseWriter, r *http.Request) {
	currentUser := s.getLoggedInUser(r)
	if currentUser == nil || !currentUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ServerConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.store.UpdateConfig(req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Dynamically control LLM server execution state
	if req.EnableLlamafile {
		s.StartLlamafile()
	} else {
		s.StopLlamafile()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func (s *Server) apiVersionCheck(w http.ResponseWriter, r *http.Request) {
	clientVer := r.URL.Query().Get("version")
	serverVer := config.Version

	isNewer := isVersionNewer(clientVer, serverVer)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"server_version": serverVer,
		"is_newer":       isNewer,
	})
}

func isVersionNewer(clientVer, serverVer string) bool {
	clientVer = strings.TrimPrefix(clientVer, "v")
	serverVer = strings.TrimPrefix(serverVer, "v")

	cParts := strings.Split(clientVer, ".")
	sParts := strings.Split(serverVer, ".")

	for i := 0; i < 3; i++ {
		cVal := 0
		sVal := 0
		if i < len(cParts) {
			fmt.Sscanf(cParts[i], "%d", &cVal)
		}
		if i < len(sParts) {
			fmt.Sscanf(sParts[i], "%d", &sVal)
		}
		if sVal > cVal {
			return true
		}
		if cVal > sVal {
			return false
		}
	}
	return false
}

func (s *Server) handleUninstallSh(w http.ResponseWriter, r *http.Request) {
	s.trackEvent(r, "link", "download", "uninstall.sh")
	script := `#!/bin/sh
set -e

echo "--- Promptyly Uninstaller (Unix/macOS) ---"

# 1. Stop daemon if running
echo "🔌 Stopping background daemon..."
if command -v pkill >/dev/null 2>&1; then
    pkill -f "promptyly serve" || true
else
    PID=$(ps aux | grep "[p]romptyly serve" | grep -v grep | awk '{print $2}')
    if [ -n "$PID" ]; then
        kill "$PID" || true
    fi
fi

# 2. Run unregister using the local binary if it exists
INSTALL_DIR="${HOME}/.local/bin"
if [ -d "/data/data/com.termux" ] || [ "$(uname -o 2>/dev/null)" = "Android" ]; then
    if [ -n "${PREFIX}" ]; then
        INSTALL_DIR="${PREFIX}/bin"
    fi
fi
INSTALL_PATH="${INSTALL_DIR}/promptyly"

if [ -f "${INSTALL_PATH}" ]; then
    echo "⚙️ Unregistering custom protocol URL scheme..."
    "${INSTALL_PATH}" unregister || true
fi

# 3. Remove systemd service
if command -v systemctl >/dev/null 2>&1; then
    echo "⚙️ Removing systemd user service..."
    systemctl --user stop promptyly.service 2>/dev/null || true
    systemctl --user disable promptyly.service 2>/dev/null || true
    rm -f "${HOME}/.config/systemd/user/promptyly.service"
    systemctl --user daemon-reload 2>/dev/null || true
fi

# 4. Remove launchd plist (macOS)
if [ "$(uname -s | tr '[:upper:]' '[:lower:]')" = "darwin" ]; then
    echo "⚙️ Removing launchd agent..."
    launchctl unload "${HOME}/Library/LaunchAgents/com.promptyly.daemon.plist" 2>/dev/null || true
    rm -f "${HOME}/Library/LaunchAgents/com.promptyly.daemon.plist"
fi

# 5. Delete binary
if [ -f "${INSTALL_PATH}" ]; then
    rm -f "${INSTALL_PATH}"
    echo "✓ Deleted promptyly executable."
fi

# 6. Ask for configuration cleanup (Interactive)
printf "\n❓ Do you want to delete configuration files (API keys, etc.) in ~/.config/promptyly? (y/N): "
read CONF_ANS < /dev/tty
if [ "$CONF_ANS" = "y" ] || [ "$CONF_ANS" = "Y" ] || [ "$CONF_ANS" = "yes" ]; then
    rm -rf "${HOME}/.config/promptyly"
    echo "✓ Configuration directory removed."
fi

# 7. Ask for data cleanup
printf "❓ Do you want to delete all downloaded/generated web apps in ~/promptyly-apps? (y/N): "
read DATA_ANS < /dev/tty
if [ "$DATA_ANS" = "y" ] || [ "$DATA_ANS" = "Y" ] || [ "$DATA_ANS" = "yes" ]; then
    rm -rf "${HOME}/promptyly-apps"
    echo "✓ Data directory removed."
fi

# 8. Ask for llamafile models cleanup
printf "❓ Do you want to delete downloaded local llamafile models in ~/.local/share/promptyly? (y/N): "
read MODEL_ANS < /dev/tty
if [ "$MODEL_ANS" = "y" ] || [ "$MODEL_ANS" = "Y" ] || [ "$MODEL_ANS" = "yes" ]; then
    rm -rf "${HOME}/.local/share/promptyly"
    echo "✓ Local models directory removed."
fi

echo ""
echo "🎉 Promptyly has been successfully uninstalled from your system!"
`
	w.Header().Set("Content-Type", "text/x-sh")
	_, _ = w.Write([]byte(script))
}

func (s *Server) handleUninstallPs1(w http.ResponseWriter, r *http.Request) {
	s.trackEvent(r, "link", "download", "uninstall.ps1")
	script := `$ErrorActionPreference = "Stop"

Write-Host "--- Promptyly Uninstaller (Windows) ---" -ForegroundColor Cyan

# 1. Stop background daemon processes
Write-Host "🔌 Stopping background daemon..." -ForegroundColor Yellow
$process = Get-Process -Name "promptyly" -ErrorAction SilentlyContinue
if ($process) {
    Stop-Process -Name "promptyly" -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
}

# 2. Run unregister using local binary if it exists
$installDir = Join-Path $HOME ".local\bin"
$installPath = Join-Path $installDir "promptyly.exe"

if (Test-Path $installPath) {
    Write-Host "⚙️ Unregistering custom protocol URL scheme..." -ForegroundColor Yellow
    try {
        & $installPath unregister
    } catch {}
}

# 3. Stop and remove Scheduled Task
Write-Host "⚙️ Removing Scheduled Task..." -ForegroundColor Yellow
try {
    $taskName = "PromptylyDaemon"
    Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue | Unregister-ScheduledTask -Confirm:$false
    Write-Host "✓ Removed Windows Scheduled Task." -ForegroundColor Green
} catch {
    Write-Warning "⚠️ Failed to remove Scheduled Task."
}

# 4. Remove installDir from User PATH environment variable
$userPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -like "*$installDir*") {
    $newUserPath = $userPath -replace [regex]::Escape($installDir), "" -replace ";+", ";" -replace "^;|;$", ""
    [System.Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
    Write-Host "✓ Removed $installDir from User PATH." -ForegroundColor Yellow
}

# 5. Delete binary
if (Test-Path $installPath) {
    Start-Sleep -Milliseconds 500
    try {
        Remove-Item -Path $installPath -Force
        Write-Host "✓ Deleted promptyly executable." -ForegroundColor Green
    } catch {
        Write-Warning "⚠️ Executable is locked. To complete uninstallation, delete it manually at: $installPath"
    }
}

# 6. Ask for configuration cleanup (Interactive)
Write-Host ""
$confAns = Read-Host "❓ Do you want to delete configuration files (API keys, etc.) in ~/.config/promptyly? (y/N)"
if ($confAns -eq "y" -or $confAns -eq "yes") {
    $configDir = Join-Path $HOME ".config\promptyly"
    if (Test-Path $configDir) {
        Remove-Item -Path $configDir -Recurse -Force
        Write-Host "✓ Configuration directory removed." -ForegroundColor Green
    }
}

# 7. Ask for data cleanup
$dataAns = Read-Host "❓ Do you want to delete all downloaded/generated web apps in ~/promptyly-apps? (y/N)"
if ($dataAns -eq "y" -or $dataAns -eq "yes") {
    $appsDir = Join-Path $HOME "promptyly-apps"
    if (Test-Path $appsDir) {
        Remove-Item -Path $appsDir -Recurse -Force
        Write-Host "✓ Data directory removed." -ForegroundColor Green
    }
}

# 8. Ask for llamafile models cleanup
$modelAns = Read-Host "❓ Do you want to delete downloaded local llamafile models in ~/.local/share/promptyly? (y/N)"
if ($modelAns -eq "y" -or $modelAns -eq "yes") {
    $modelsDir = Join-Path $HOME ".local\share\promptyly"
    if (Test-Path $modelsDir) {
        Remove-Item -Path $modelsDir -Recurse -Force
        Write-Host "✓ Local models directory removed." -ForegroundColor Green
    }
}

Write-Host ""
Write-Host "🎉 Promptyly has been successfully uninstalled from your system!" -ForegroundColor Green
`
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(script))
}

func (s *Server) apiCreateApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := s.getAPIUser(r)
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unauthorized: valid session cookie or API token required",
		})
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if strings.TrimSpace(req.Prompt) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Prompt cannot be empty",
		})
		return
	}

	// Check if llamafile server is running inside container/server on port 6080
	conn, err := net.DialTimeout("tcp", "127.0.0.1:6080", 1*time.Second)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "LLM generation server is not running or still starting up",
		})
		return
	}
	conn.Close()

	s.trackEvent(r, "app", "generate", req.Prompt)

	// Slugify prompt to get a safe name
	appName := slugify(req.Prompt)
	if len(appName) == 0 {
		appName = "generated-app"
	}
	
	// Append random timestamp key to name to avoid directory clashes
	appName = fmt.Sprintf("%s-%d", appName, time.Now().Unix()%1000)

	// Set up agent client
	provCfg := config.ProviderConfig{
		Type:  "openai-compatible",
		URL:   "http://127.0.0.1:6080/v1",
		Model: "qwen2.5-coder-1.5b-instruct",
	}
	client, err := agent.NewClient("llamafile", provCfg)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	// Define system prompt
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
`

	resp, err := client.Generate(r.Context(), systemPrompt, req.Prompt, nil)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "LLM generation failed: " + err.Error()})
		return
	}

	if len(resp.Files) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Agent failed to generate any files."})
		return
	}

	// Create zip file
	zipFilename := fmt.Sprintf("%d-promptyly-pub-%s.zip", time.Now().UnixNano(), appName)
	zipPath := filepath.Join(s.dataDir, "zips", zipFilename)
	
	zipFile, err := os.Create(zipPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Failed to create ZIP: " + err.Error()})
		return
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	for filename, content := range resp.Files {
		f, err := zipWriter.Create(filename)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		_, _ = f.Write([]byte(content))
	}
	
	// Add readme
	f, err := zipWriter.Create("README.md")
	if err == nil {
		displayName := strings.Title(strings.ReplaceAll(appName, "-", " "))
		readme := fmt.Sprintf("# %s\n\nGenerated directly on Registry Server.\nPrompt: %s\n", displayName, req.Prompt)
		_, _ = f.Write([]byte(readme))
	}

	if err := zipWriter.Close(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Failed to finalise ZIP: " + err.Error()})
		return
	}

	// Serve the app statically by writing it to s.dataDir/apps/appName
	appFolder := filepath.Join(s.dataDir, "apps", appName)
	_ = os.MkdirAll(appFolder, 0755)
	for filename, content := range resp.Files {
		fullPath := filepath.Join(appFolder, filename)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		_ = os.WriteFile(fullPath, []byte(content), 0644)
	}

	if _, err := s.store.AddApp(user.ID, user.Username, appName, req.Prompt, "Generated directly on Sharing Registry server.", zipFilename); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Failed to save to database: " + err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"appName": appName,
		"url":     "/apps/" + appName + "/",
	})
}

func sortApps(apps []*App, sortBy, order string) {
	sort.Slice(apps, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "views":
			less = apps[i].Views < apps[j].Views
		case "created_by":
			less = strings.ToLower(apps[i].Username) < strings.ToLower(apps[j].Username)
		case "name":
			less = strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
		case "upload_date":
			fallthrough
		default:
			less = apps[i].CreatedAt.Before(apps[j].CreatedAt)
		}
		if order == "desc" {
			return !less
		}
		return less
	})
}


