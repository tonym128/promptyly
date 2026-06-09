package git

import (
	"bytes"
	"os/exec"
	"strings"
)

// RunCommand runs a git command in the specified directory.
func RunCommand(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Init initializes a git repository in the target directory.
func Init(dir string) error {
	_, err := RunCommand(dir, "init")
	if err != nil {
		return err
	}

	// Set local git config in case global config is missing, to ensure commits work
	_, _ = RunCommand(dir, "config", "user.name", "Promptyly")
	_, _ = RunCommand(dir, "config", "user.email", "agent@promptyly.local")

	return nil
}

// CommitAll adds all files and commits them with the given message.
func CommitAll(dir string, message string) (string, error) {
	_, err := RunCommand(dir, "add", ".")
	if err != nil {
		return "", err
	}

	// Check if there are changes to commit
	status, err := RunCommand(dir, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if len(status) == 0 {
		return "No changes to commit", nil
	}

	return RunCommand(dir, "commit", "-m", message)
}

// GetLog returns the commit history of the repository.
func GetLog(dir string) (string, error) {
	return RunCommand(dir, "log", "--oneline", "-n", "20")
}
