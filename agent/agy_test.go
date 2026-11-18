package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopySeedFilesAndCollectChanges(t *testing.T) {
	seedDir := t.TempDir()
	workDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(seedDir, "index.html"), []byte("<h1>Old</h1>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "style.css"), []byte("body {}"), 0644); err != nil {
		t.Fatal(err)
	}

	hashes, err := copySeedFiles(seedDir, workDir)
	if err != nil {
		t.Fatalf("copySeedFiles failed: %v", err)
	}
	if len(hashes) != 2 {
		t.Fatalf("expected 2 hashed files, got %d", len(hashes))
	}

	// Modify index.html and add game.js; leave style.css unchanged
	if err := os.WriteFile(filepath.Join(workDir, "index.html"), []byte("<h1>New</h1>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "game.js"), []byte("console.log(1);"), 0644); err != nil {
		t.Fatal(err)
	}

	changed, err := collectWorkspaceChanges(workDir, hashes)
	if err != nil {
		t.Fatalf("collectWorkspaceChanges failed: %v", err)
	}
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed files, got %d: %v", len(changed), changed)
	}
	if changed["index.html"] != "<h1>New</h1>" {
		t.Errorf("unexpected index.html content: %q", changed["index.html"])
	}
	if changed["game.js"] != "console.log(1);" {
		t.Errorf("unexpected game.js content: %q", changed["game.js"])
	}
	if _, ok := changed["style.css"]; ok {
		t.Errorf("unchanged style.css should not be in changed map")
	}
}
