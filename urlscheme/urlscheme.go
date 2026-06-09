package urlscheme

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Register registers the prompt:// custom URL scheme handler on the host OS.
func Register() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %v", err)
	}

	switch runtime.GOOS {
	case "linux":
		return registerLinux(execPath)
	case "windows":
		return registerWindows(execPath)
	case "darwin":
		return registerMacOS(execPath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func registerLinux(execPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	appsDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		return err
	}

	desktopFile := filepath.Join(appsDir, "promptyly-url-handler.desktop")
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Promptyly URL Handler
Exec=%s handle %%u
StartupNotify=true
Terminal=true
MimeType=x-scheme-handler/prompt;
NoDisplay=true
`, execPath)

	if err := os.WriteFile(desktopFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write desktop file: %v", err)
	}

	// Update desktop database
	cmd := exec.Command("update-desktop-database", appsDir)
	_ = cmd.Run()

	// Set as default scheme handler
	cmd = exec.Command("xdg-mime", "default", "promptyly-url-handler.desktop", "x-scheme-handler/prompt")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to register mime type with xdg-mime: %v", err)
	}

	fmt.Println("Successfully registered prompt:// URL scheme handler on Linux.")
	return nil
}

func registerWindows(execPath string) error {
	// On Windows, we can use the reg.exe command to add keys without importing golang.org/x/sys/windows/registry,
	// keeping build configuration simple and compiling on all operating systems.
	escapedExec := fmt.Sprintf(`\"%s\" handle \"%%1\"`, execPath)

	commands := [][]string{
		{"add", "HKCR\\prompt", "/ve", "/d", "URL:Promptyly Protocol", "/f"},
		{"add", "HKCR\\prompt", "/v", "URL Protocol", "/d", "", "/f"},
		{"add", "HKCR\\prompt\\shell\\open\\command", "/ve", "/d", escapedExec, "/f"},
	}

	for _, args := range commands {
		cmd := exec.Command("reg", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run registry update command: reg %v", args)
		}
	}

	fmt.Println("Successfully registered prompt:// URL scheme handler in Windows registry.")
	return nil
}

func registerMacOS(execPath string) error {
	// macOS registers custom schemes via App Bundles (.app) and Info.plist.
	// Since we are running a CLI binary, we guide the user on how macOS protocol handlers work.
	fmt.Printf("Custom URL protocol registration on macOS requires packaging the CLI as an App Bundle.\n")
	fmt.Printf("To run prompts directly, use: promptyly handle \"prompt://create?prompt=...\"\n")
	fmt.Printf("Or run the CLI commands directly: promptyly create \"...\"\n")
	return nil
}
