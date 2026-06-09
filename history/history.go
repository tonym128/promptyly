package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type ActionEntry struct {
	Timestamp     time.Time `json:"timestamp"`
	Action        string    `json:"action"` // "create", "edit"
	Prompt        string    `json:"prompt"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	FilesAffected []string  `json:"files_affected"`
	Summary       string    `json:"summary"`
}

type History []ActionEntry

func GetHistoryPath(appDir string) string {
	return filepath.Join(appDir, ".promptyly", "history.json")
}

func LoadHistory(appDir string) (History, error) {
	path := GetHistoryPath(appDir)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return History{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var h History
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, err
	}

	return h, nil
}

func SaveHistory(appDir string, h History) error {
	path := GetHistoryPath(appDir)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func AddEntry(appDir string, entry ActionEntry) error {
	h, err := LoadHistory(appDir)
	if err != nil {
		return err
	}

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	h = append(h, entry)
	return SaveHistory(appDir, h)
}
