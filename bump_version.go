package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	versionFile := "version.txt"
	defaultVersion := "0.2.10"

	// Read existing version or use default
	content, err := os.ReadFile(versionFile)
	versionStr := defaultVersion
	if err == nil {
		versionStr = strings.TrimSpace(string(content))
	}

	parts := strings.Split(versionStr, ".")
	if len(parts) != 3 {
		// Fallback to default if malformed
		parts = strings.Split(defaultVersion, ".")
	}

	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	revision, err3 := strconv.Atoi(parts[2])

	if err1 != nil || err2 != nil || err3 != nil {
		major = 0
		minor = 2
		revision = 10
	} else {
		// Increment revision
		revision++
	}

	newVersionStr := fmt.Sprintf("%d.%d.%d", major, minor, revision)

	// Save to version.txt
	err = os.WriteFile(versionFile, []byte(newVersionStr), 0644)
	if err != nil {
		fmt.Printf("Error writing version.txt: %v\n", err)
		os.Exit(1)
	}

	// Generate config/version.go
	configDir := "config"
	versionGoPath := filepath.Join(configDir, "version.go")
	versionGoContent := fmt.Sprintf(`package config

// Version is the auto-incremented version number of Promptyly
const Version = "%s"
`, newVersionStr)

	err = os.WriteFile(versionGoPath, []byte(versionGoContent), 0644)
	if err != nil {
		fmt.Printf("Error writing config/version.go: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Version updated: %s -> %s\n", versionStr, newVersionStr)
}
