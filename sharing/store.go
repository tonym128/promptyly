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

type Store struct {
	mu       sync.RWMutex
	filePath string
	Users    map[string]*User `json:"users"`  // username -> User
	Tokens   map[string]string `json:"tokens"` // token -> username
	Apps     map[string]*App  `json:"apps"`   // id -> App
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
