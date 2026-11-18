package app

import (
	"archive/zip"
	"os"
	"path/filepath"
	"promptyly/config"
	"promptyly/history"
	"strings"
	"testing"
)

func TestExportAndImportAppRoundTrip(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	appName := "velvet-pour"
	appDir := filepath.Join(cfg.AppsDir, appName)
	if err := os.MkdirAll(filepath.Join(appDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	origHTML := "<!doctype html><html><body><h1>Velvet Pour</h1></body></html>"
	if err := os.WriteFile(filepath.Join(appDir, "index.html"), []byte(origHTML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "game.js"), []byte("console.log('hi');"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := history.AddEntry(appDir, history.ActionEntry{
		Action: "create",
		Prompt: "a game about sliding drinks",
	}); err != nil {
		t.Fatal(err)
	}

	cfg.Apps[appName] = appDir
	cfg.Prompts["a game about sliding drinks"] = appName

	zipPath := filepath.Join(t.TempDir(), "velvet-pour.zip")
	if err := ExportApp(cfg, appName, zipPath); err != nil {
		t.Fatalf("ExportApp failed: %v", err)
	}

	// Verify exported zip contents
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("OpenReader failed: %v", err)
	}
	seen := make(map[string]bool)
	for _, f := range zr.File {
		seen[f.Name] = true
		if strings.HasPrefix(f.Name, ".git") {
			t.Errorf("exported zip should not contain .git: %s", f.Name)
		}
	}
	zr.Close()
	for _, req := range []string{"index.html", "game.js", "play.sh", "play.bat"} {
		if !seen[req] {
			t.Errorf("expected %s in exported zip", req)
		}
	}

	// Import into a fresh apps directory
	cfg2, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg2.AppsDir = filepath.Join(t.TempDir(), "apps")
	cfg2.Apps = make(map[string]string)
	cfg2.Prompts = make(map[string]string)

	importedName, err := ImportApp(cfg2, zipPath)
	if err != nil {
		t.Fatalf("ImportApp failed: %v", err)
	}
	if importedName != "velvet-pour" {
		t.Errorf("expected importedName=velvet-pour, got %q", importedName)
	}

	importedDir := cfg2.Apps[importedName]
	gotHTML, err := os.ReadFile(filepath.Join(importedDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(gotHTML), offlineShimStart) {
		t.Errorf("imported index.html should strip offline shim")
	}
	if _, err := os.Stat(filepath.Join(importedDir, "play.sh")); !os.IsNotExist(err) {
		t.Errorf("imported dir should not contain play.sh")
	}
	if cfg2.Prompts["a game about sliding drinks"] != importedName {
		t.Errorf("expected prompt mapping restored, got %v", cfg2.Prompts)
	}
}
