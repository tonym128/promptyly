package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Salt         string    `json:"salt"`
	Token        string    `json:"token"`
	IsAdmin      bool      `json:"is_admin"`
	IsApproved   bool      `json:"is_approved"`
	CreatedAt    time.Time `json:"created_at"`
}

type App struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	Name        string    `json:"name"`
	Prompt      string    `json:"prompt"`
	Description string    `json:"description"`
	ZipName     string    `json:"zip_name"`
	Views       int       `json:"views"`
	Downloads   int       `json:"downloads"`
	CreatedAt   time.Time `json:"created_at"`
}

type ServerConfig struct {
	RequireAdminApproval  bool `json:"require_admin_approval"`
	RequireLoginToView    bool `json:"require_login_to_view"`
	AllowSelfRegistration bool `json:"allow_self_registration"`
}

type AnalyticsEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Category  string    `json:"category"`  // "page", "link", "app", "upload", "url"
	Action    string    `json:"action"`    // "view", "click", "download", "publish", "create"
	Label     string    `json:"label"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
}

type Store struct {
	mu        sync.RWMutex
	filePath  string
	Users     map[string]*User           `json:"users"`  // username -> User
	Tokens    map[string]string          `json:"tokens"` // token -> username
	Apps      map[string]*App            `json:"apps"`   // id -> App
	Config    ServerConfig               `json:"config"`
	Analytics []AnalyticsEvent           `json:"analytics,omitempty"`
}

func NewStore(filePath string) (*Store, error) {
	s := &Store{
		filePath: filePath,
		Users:    make(map[string]*User),
		Tokens:   make(map[string]string),
		Apps:     make(map[string]*App),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		// Create parent directory if needed
		if err := os.MkdirAll(filepath.Dir(s.filePath), 0755); err != nil {
			return err
		}
		// Initialize default config from env
		s.Config.RequireAdminApproval = os.Getenv("REQUIRE_ADMIN_APPROVAL") == "true"
		s.Config.RequireLoginToView = os.Getenv("REQUIRE_LOGIN_TO_VIEW") == "true"
		s.Config.AllowSelfRegistration = os.Getenv("ALLOW_SELF_REGISTRATION") != "false"
		return s.saveLocked()
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var temp struct {
		Users  map[string]*User  `json:"users"`
		Tokens map[string]string `json:"tokens"`
		Apps   map[string]*App   `json:"apps"`
		Config *ServerConfig     `json:"config"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	if temp.Users != nil {
		s.Users = temp.Users
	}
	if temp.Tokens != nil {
		s.Tokens = temp.Tokens
	}
	if temp.Apps != nil {
		s.Apps = temp.Apps
	}
	if temp.Config != nil {
		s.Config = *temp.Config
	} else {
		// Fallback/Initialize from environment variables if not present in file
		s.Config.RequireAdminApproval = os.Getenv("REQUIRE_ADMIN_APPROVAL") == "true"
		s.Config.RequireLoginToView = os.Getenv("REQUIRE_LOGIN_TO_VIEW") == "true"
		s.Config.AllowSelfRegistration = os.Getenv("ALLOW_SELF_REGISTRATION") != "false"
	}

	return nil
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func generateRandomString(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func hashPassword(password, salt string) string {
	h := sha256.New()
	h.Write([]byte(password + salt))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Store) RegisterUser(username, password string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	usernameNorm := strings.ToLower(strings.TrimSpace(username))
	if usernameNorm == "" {
		return nil, errors.New("username cannot be empty")
	}
	if len(password) < 6 {
		return nil, errors.New("password must be at least 6 characters")
	}

	if _, exists := s.Users[usernameNorm]; exists {
		return nil, fmt.Errorf("username '%s' is already registered", username)
	}

	salt := generateRandomString(16)
	token := generateRandomString(32)

	user := &User{
		ID:           generateRandomString(8),
		Username:     username,
		PasswordHash: hashPassword(password, salt),
		Salt:         salt,
		Token:        token,
		IsAdmin:      false,
		IsApproved:   !s.Config.RequireAdminApproval,
		CreatedAt:    time.Now(),
	}

	s.Users[usernameNorm] = user
	s.Tokens[token] = usernameNorm

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Store) LoginUser(username, password string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	usernameNorm := strings.ToLower(strings.TrimSpace(username))
	user, exists := s.Users[usernameNorm]
	if !exists {
		return "", errors.New("invalid username or password")
	}

	expectedHash := hashPassword(password, user.Salt)
	if user.PasswordHash != expectedHash {
		return "", errors.New("invalid username or password")
	}

	if !user.IsApproved && !user.IsAdmin {
		return "", errors.New("account pending admin approval")
	}

	// Rotate or refresh token
	if user.Token == "" {
		user.Token = generateRandomString(32)
	}
	s.Tokens[user.Token] = usernameNorm

	if err := s.saveLocked(); err != nil {
		return "", err
	}

	return user.Token, nil
}

func (s *Store) GetUserByToken(token string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	usernameNorm, exists := s.Tokens[token]
	if !exists {
		return nil, false
	}

	user, exists := s.Users[usernameNorm]
	return user, exists
}

func slugify(s string) string {
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

func (s *Store) AddApp(userID, username, name, prompt, description, zipName string) (*App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		cleanName = slugify(prompt)
	}

	appID := slugify(cleanName)
	if _, exists := s.Apps[appID]; exists {
		// Resolve collisions by appending random suffix
		appID = fmt.Sprintf("%s-%s", appID, generateRandomString(4))
	}

	app := &App{
		ID:          appID,
		UserID:      userID,
		Username:    username,
		Name:        cleanName,
		Prompt:      prompt,
		Description: description,
		ZipName:     zipName,
		Views:       0,
		Downloads:   0,
		CreatedAt:   time.Now(),
	}

	s.Apps[app.ID] = app

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	return app, nil
}

func (s *Store) ListApps() []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*App
	for _, app := range s.Apps {
		list = append(list, app)
	}

	// Sort by newest first
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[i].CreatedAt.Before(list[j].CreatedAt) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}

	return list
}

func (s *Store) SearchApps(query string) []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.ToLower(strings.TrimSpace(query))
	var results []*App
	for _, app := range s.Apps {
		if query == "" ||
			strings.Contains(strings.ToLower(app.Name), query) ||
			strings.Contains(strings.ToLower(app.Prompt), query) ||
			strings.Contains(strings.ToLower(app.Description), query) ||
			strings.Contains(strings.ToLower(app.Username), query) {
			results = append(results, app)
		}
	}

	// Sort by newest first
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].CreatedAt.Before(results[j].CreatedAt) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

func (s *Store) GetApp(id string) (*App, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	app, exists := s.Apps[id]
	return app, exists
}

func (s *Store) DeleteApp(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Apps[id]; !exists {
		return fmt.Errorf("app '%s' not found", id)
	}

	delete(s.Apps, id)
	return s.saveLocked()
}

func (s *Store) IncrementViews(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if app, exists := s.Apps[id]; exists {
		app.Views++
		_ = s.saveLocked()
	}
}

func (s *Store) IncrementDownloads(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if app, exists := s.Apps[id]; exists {
		app.Downloads++
		_ = s.saveLocked()
	}
}

func (s *Store) EnsureAdminUser(envUser, envPass string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if an admin already exists
	adminExists := false
	for _, user := range s.Users {
		if user.IsAdmin {
			adminExists = true
			break
		}
	}

	if adminExists {
		return "", "", nil
	}

	// No admin exists, create one
	username := "admin"
	if envUser != "" {
		username = envUser
	}

	usernameNorm := strings.ToLower(strings.TrimSpace(username))
	
	// If the user already exists, we make them admin. Otherwise we create them.
	user, exists := s.Users[usernameNorm]
	var password string
	if exists {
		user.IsAdmin = true
		user.IsApproved = true
	} else {
		password = envPass
		if password == "" {
			password = generateRandomString(12) // Generates 24 hex characters
		}
		salt := generateRandomString(16)
		token := generateRandomString(32)

		user = &User{
			ID:           generateRandomString(8),
			Username:     username,
			PasswordHash: hashPassword(password, salt),
			Salt:         salt,
			Token:        token,
			IsAdmin:      true,
			IsApproved:   true,
			CreatedAt:    time.Now(),
		}
		s.Users[usernameNorm] = user
		s.Tokens[token] = usernameNorm
	}

	if err := s.saveLocked(); err != nil {
		return "", "", err
	}

	return username, password, nil
}

func (s *Store) ListUsers() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*User
	for _, user := range s.Users {
		list = append(list, user)
	}
	return list
}

func (s *Store) ApproveUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	usernameNorm := strings.ToLower(strings.TrimSpace(username))
	user, exists := s.Users[usernameNorm]
	if !exists {
		return fmt.Errorf("user '%s' not found", username)
	}

	user.IsApproved = true
	return s.saveLocked()
}

func (s *Store) RejectUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	usernameNorm := strings.ToLower(strings.TrimSpace(username))
	_, exists := s.Users[usernameNorm]
	if !exists {
		return fmt.Errorf("user '%s' not found", username)
	}

	// Delete from Users and Tokens
	delete(s.Users, usernameNorm)
	for t, u := range s.Tokens {
		if u == usernameNorm {
			delete(s.Tokens, t)
		}
	}
	return s.saveLocked()
}

func (s *Store) GetConfig() ServerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Config
}

func (s *Store) UpdateConfig(config ServerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Config = config
	return s.saveLocked()
}

func (s *Store) RecordEvent(category, action, label string, ip string, ua string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	event := AnalyticsEvent{
		Timestamp: time.Now(),
		Category:  category,
		Action:    action,
		Label:     label,
		IP:        ip,
		UserAgent: ua,
	}

	s.Analytics = append(s.Analytics, event)

	// Cap the log at 5000 events to prevent JSON file bloating
	if len(s.Analytics) > 5000 {
		s.Analytics = s.Analytics[len(s.Analytics)-5000:]
	}

	_ = s.saveLocked()
}

func (s *Store) GetAnalytics() []AnalyticsEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Return a copy to avoid slice sharing data race
	res := make([]AnalyticsEvent, len(s.Analytics))
	copy(res, s.Analytics)
	return res
}
