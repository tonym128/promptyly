package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	portFlag := flag.Int("port", 6072, "Port to run the sharing server on")
	dataDirFlag := flag.String("data", "./data", "Directory to store databases and app zips")
	flag.Parse()

	// Allow environment variables to override flags
	port := *portFlag
	if envPort := os.Getenv("PORT"); envPort != "" {
		var p int
		if _, err := fmt.Sscanf(envPort, "%d", &p); err == nil {
			port = p
		}
	}

	dataDir := *dataDirFlag
	if envDataDir := os.Getenv("DATA_DIR"); envDataDir != "" {
		dataDir = envDataDir
	}

	// Resolve absolute path for data directory
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		fmt.Printf("❌ Error resolving absolute data directory path: %v\n", err)
		os.Exit(1)
	}

	// Create directories
	dirs := []string{
		absDataDir,
		filepath.Join(absDataDir, "zips"),
		filepath.Join(absDataDir, "apps"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Printf("❌ Failed to create directory '%s': %v\n", dir, err)
			os.Exit(1)
		}
	}

	dbFilePath := filepath.Join(absDataDir, "store.json")
	store, err := NewStore(dbFilePath)
	if err != nil {
		fmt.Printf("❌ Failed to initialize database store: %v\n", err)
		os.Exit(1)
	}

	server := NewServer(store, absDataDir)
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("==================================================\n")
	fmt.Printf("🚀 Promptyly Sharing Server running on http://localhost%s\n", addr)
	fmt.Printf("📁 Data Directory: %s\n", absDataDir)
	fmt.Printf("==================================================\n")

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("❌ Server failed to start: %v\n", err)
		os.Exit(1)
	}
}
