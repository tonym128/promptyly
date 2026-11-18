package agent

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"promptyly/config"
	"strings"
)

// AgyClient invokes the Antigravity CLI (`agy`) in non-interactive print mode.
type AgyClient struct {
	config config.ProviderConfig
}

func NewAgyClient(cfg config.ProviderConfig) *AgyClient {
	return &AgyClient{config: cfg}
}

func findAgyBinary() (string, error) {
	if p, err := exec.LookPath("agy"); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		"/usr/local/bin/agy",
		"/usr/bin/agy",
	}
	if home != "" {
		candidates = append([]string{filepath.Join(home, ".local", "bin", "agy")}, candidates...)
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("agy CLI binary not found in PATH or ~/.local/bin/agy")
}

type agyStreamEvent struct {
	Event      string `json:"event"`
	StepUpdate *struct {
		StepIndex int    `json:"step_index"`
		State     string `json:"state"`
		StepType  string `json:"step_type"`
		TextDelta string `json:"text_delta"`
	} `json:"step_update,omitempty"`
	Result *struct {
		Status   string `json:"status"`
		Response string `json:"response"`
		Error    string `json:"error,omitempty"`
	} `json:"result,omitempty"`
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func shouldSkipWorkspaceEntry(name string) bool {
	return name == ".git" || name == ".promptyly" || name == "node_modules" || name == "response.txt"
}

func copySeedFiles(srcDir, dstDir string) (map[string]string, error) {
	hashes := make(map[string]string)
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != srcDir && shouldSkipWorkspaceEntry(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldSkipWorkspaceEntry(info.Name()) {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(dstDir, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		hashes[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	return hashes, err
}

func collectWorkspaceChanges(workDir string, initialHashes map[string]string) (map[string]string, error) {
	changed := make(map[string]string)
	err := filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != workDir && shouldSkipWorkspaceEntry(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldSkipWorkspaceEntry(info.Name()) {
			return nil
		}
		rel, err := filepath.Rel(workDir, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		currentHash, err := hashFile(path)
		if err != nil {
			return nil
		}
		if prevHash, existed := initialHashes[relSlash]; existed && prevHash == currentHash {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		changed[relSlash] = string(data)
		return nil
	})
	return changed, err
}

func (a *AgyClient) Generate(ctx context.Context, systemPrompt, userPrompt string, onToken func(token string)) (*Response, error) {
	binPath, err := findAgyBinary()
	if err != nil {
		return nil, err
	}

	model := a.config.Model
	if model == "" {
		model = "gemini-4-hydrogen-medium"
	}
	effort := a.config.Effort
	if effort == "" {
		effort = "medium"
	}

	workDir, err := os.MkdirTemp("", "promptyly-agy-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create agy workspace: %v", err)
	}
	defer os.RemoveAll(workDir)

	initialHashes := make(map[string]string)
	if seedDir := SeedDirFromContext(ctx); seedDir != "" {
		if hashes, err := copySeedFiles(seedDir, workDir); err == nil {
			initialHashes = hashes
		}
	}

	var promptBuilder strings.Builder
	if strings.TrimSpace(systemPrompt) != "" {
		promptBuilder.WriteString(systemPrompt)
		promptBuilder.WriteString("\n\n---\n\n")
	}
	promptBuilder.WriteString("User Request:\n")
	promptBuilder.WriteString(userPrompt)

	args := []string{
		"-p", promptBuilder.String(),
		"--model", model,
		"--effort", effort,
		"--mode", "auto",
		"--dangerously-skip-permissions",
		"--output-format", "stream-json",
		"--disable-slash-commands",
	}

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = workDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to attach stdout pipe: %v", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start agy: %v", err)
	}

	var streamedText strings.Builder
	var finalResultText string
	var resultStatus string
	var resultErr string
	lastStepIndex := -1

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var evt agyStreamEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}
		if evt.Event == "step_update" && evt.StepUpdate != nil {
			if evt.StepUpdate.StepIndex != lastStepIndex && evt.StepUpdate.StepType != "" && evt.StepUpdate.StepType != "user_input" && evt.StepUpdate.StepType != "agent_response" {
				lastStepIndex = evt.StepUpdate.StepIndex
				if onToken != nil {
					onToken(fmt.Sprintf("[agy: %s]\n", evt.StepUpdate.StepType))
				}
			}
			if evt.StepUpdate.TextDelta != "" {
				streamedText.WriteString(evt.StepUpdate.TextDelta)
				if onToken != nil {
					onToken(evt.StepUpdate.TextDelta)
				}
			}
		} else if evt.Event == "result" && evt.Result != nil {
			resultStatus = evt.Result.Status
			finalResultText = evt.Result.Response
			resultErr = evt.Result.Error
		}
	}

	waitErr := cmd.Wait()
	if waitErr != nil {
		errMsg := strings.TrimSpace(stderrBuf.String())
		if resultErr != "" {
			errMsg = resultErr
		}
		if errMsg == "" {
			errMsg = waitErr.Error()
		}
		return nil, fmt.Errorf("agy execution failed: %s", errMsg)
	}
	if resultStatus != "" && resultStatus != "SUCCESS" {
		errMsg := resultErr
		if errMsg == "" {
			errMsg = strings.TrimSpace(stderrBuf.String())
		}
		return nil, fmt.Errorf("agy finished with status %s: %s", resultStatus, errMsg)
	}

	rawText := finalResultText
	if rawText == "" {
		rawText = streamedText.String()
	}

	// 1. Parse any <file> / <summary> tags from response text
	res := ParseResponse(rawText)

	// 2. Collect any files created or modified directly in the workspace by agy
	diskFiles, err := collectWorkspaceChanges(workDir, initialHashes)
	if err == nil && len(diskFiles) > 0 {
		for relPath, content := range diskFiles {
			res.Files[relPath] = content
		}
		if res.Summary == "" || strings.HasPrefix(res.Summary, "Updated files:") {
			if rawText != "" && !strings.Contains(rawText, "<file") {
				trimmed := strings.TrimSpace(rawText)
				if len(trimmed) > 300 {
					trimmed = trimmed[:300] + "..."
				}
				res.Summary = trimmed
			} else {
				res.Summary = "Updated files: " + strings.Join(getFileKeys(res.Files), ", ")
			}
		}
	}

	return res, nil
}
