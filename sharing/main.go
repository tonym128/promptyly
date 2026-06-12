package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

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
		filepath.Join(absDataDir, "binaries"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Printf("❌ Failed to create directory '%s': %v\n", dir, err)
			os.Exit(1)
		}
	}

	// Copy default binaries from distribution folder if they exist
	distDir := "/app/binaries_dist"
	if _, err := os.Stat(distDir); err == nil {
		files, err := os.ReadDir(distDir)
		if err == nil {
			destDir := filepath.Join(absDataDir, "binaries")
			for _, file := range files {
				if file.IsDir() {
					continue
				}
				srcPath := filepath.Join(distDir, file.Name())
				destPath := filepath.Join(destDir, file.Name())
				// Copy if not exists
				if _, err := os.Stat(destPath); os.IsNotExist(err) {
					fmt.Printf("📦 Copying bundled binary %s to registry binaries...\n", file.Name())
					if err := copyFile(srcPath, destPath); err != nil {
						fmt.Printf("⚠️ Warning: failed to copy default binary %s: %v\n", file.Name(), err)
					}
				}
			}
		}
	}

	dbFilePath := filepath.Join(absDataDir, "store.json")
	store, err := NewStore(dbFilePath)
	if err != nil {
		fmt.Printf("❌ Failed to initialize database store: %v\n", err)
		os.Exit(1)
	}

	adminUser := os.Getenv("ADMIN_USERNAME")
	adminPass := os.Getenv("ADMIN_PASSWORD")
	createdUser, createdPass, err := store.EnsureAdminUser(adminUser, adminPass)
	if err == nil && createdUser != "" {
		fmt.Printf("\n🔒 SECURITY: Generated server admin account:\n")
		fmt.Printf("   Username: %s\n", createdUser)
		if createdPass != "" {
			fmt.Printf("   Password: %s\n", createdPass)
			fmt.Printf("   ⚠️  IMPORTANT: Please record this password! It will not be printed again.\n")
		} else {
			fmt.Printf("   Password: (using configured/existing password)\n")
		}
		fmt.Printf("   To customize, set ADMIN_USERNAME and ADMIN_PASSWORD environment variables.\n\n")
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
