package server

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"promptyly/config"
	"promptyly/urlscheme"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	clients   = make(map[string]map[chan string]bool) // AppName -> client channels
	clientsMu sync.Mutex

	apiToken string

	cachedConfig   *config.Config
	cachedConfigMu sync.Mutex

	// Callbacks to decouple server from app package (avoiding cyclic imports)
	CreateAppCallback func(prompt string) (string, string, error)
	EditAppCallback   func(name, prompt string) error
	RenameAppCallback func(oldName, newName string) (string, error)
	LinkAppCallback   func(path string) (string, error)
	UnlinkAppCallback func(name string) error
	DeleteAppCallback func(name string, deleteFolder bool) error
	ExportAppCallback func(name, zipPath string) error
	UpdateMetadataCallback func(name, newName, newPrompt string) (string, error)
	ImportAppCallback func(zipPath string) (string, error)
)

func getCachedConfig() (*config.Config, error) {
	cachedConfigMu.Lock()
	defer cachedConfigMu.Unlock()
	if cachedConfig == nil {
		cfg, err := config.LoadConfig()
		if err != nil {
			return nil, err
		}
		cachedConfig = cfg
	}
	return cachedConfig, nil
}

func reloadCachedConfig() (*config.Config, error) {
	cachedConfigMu.Lock()
	defer cachedConfigMu.Unlock()
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	cachedConfig = cfg
	return cachedConfig, nil
}

// RegisterClient registers a SSE connection channel for a specific application.
func RegisterClient(appName string, ch chan string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	if clients[appName] == nil {
		clients[appName] = make(map[chan string]bool)
	}
	clients[appName][ch] = true
}

// UnregisterClient removes a SSE connection channel for a specific application.
func UnregisterClient(appName string, ch chan string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	if clients[appName] != nil {
		delete(clients[appName], ch)
		if len(clients[appName]) == 0 {
			delete(clients, appName)
		}
	}
}

// NotifyReload sends a reload event to all browser clients connected to a specific application.
func NotifyReload(appName string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	if clients[appName] != nil {
		for ch := range clients[appName] {
			select {
			case ch <- "reload":
			default:
				// Channel blocked or full, skip
			}
		}
	}
}

func injectLiveReload(html string, ssePath string) string {
	script := fmt.Sprintf(`
<!-- Injected by Promptyly Live Reload -->
<script>
(function() {
  const es = new EventSource('%s');
  es.onmessage = function(event) {
    if (event.data === 'reload') {
      console.log('[Promptyly] Reload signal received. Reloading page...');
      window.location.reload();
    }
  };
})();
</script>
`, ssePath)
	lower := strings.ToLower(html)
	idx := strings.LastIndex(lower, "</body>")
	if idx == -1 {
		return html + script
	}
	return html[:idx] + script + html[idx:]
}

func eventsHandler(appName string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	clientChan := make(chan string, 5)
	RegisterClient(appName, clientChan)
	defer UnregisterClient(appName, clientChan)

	_, _ = fmt.Fprintf(w, "data: connected\n\n")
	flusher.Flush()

	for {
		select {
		case msg := <-clientChan:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func handleDb(appDir string) http.HandlerFunc {
	dbPath := filepath.Join(appDir, ".promptyly", "db.json")
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == "GET" {
			data, err := os.ReadFile(dbPath)
			if err != nil {
				if os.IsNotExist(err) {
					_, _ = w.Write([]byte("{}"))
					return
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_, _ = w.Write(data)
			return
		}

		if r.Method == "POST" {
			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			var js json.RawMessage
			if err := json.Unmarshal(data, &js); err != nil {
				http.Error(w, "Invalid JSON format", http.StatusBadRequest)
				return
			}

			if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if err := os.WriteFile(dbPath, data, 0644); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			_, _ = w.Write([]byte(`{"status":"success"}`))
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	cfg, err := getCachedConfig()
	if err != nil {
		http.Error(w, "Failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	appPrompts := make(map[string]string)
	for pr, name := range cfg.Prompts {
		appPrompts[name] = pr
	}

	var appNames []string
	for name := range cfg.Apps {
		appNames = append(appNames, name)
	}
	sort.Strings(appNames)
	localAppsJSON, _ := json.Marshal(appNames)

	gridHTML := ""
	if len(appNames) == 0 {
		gridHTML = `
        <div class="empty-state">
            <h3>No applications found</h3>
            <p>Generate your first application using the form above!</p>
        </div>`
	} else {
		gridHTML = `<div class="grid">`
		for _, name := range appNames {
			promptText := appPrompts[name]
			if promptText == "" {
				promptText = "Generated web application."
			}
			displayPrompt := promptText
			if len(displayPrompt) > 120 {
				displayPrompt = displayPrompt[:117] + "..."
			}

			// Format title: dash to space, capitalized
			displayName := strings.Title(strings.ReplaceAll(name, "-", " "))

			// Generate deep links
			runLink := fmt.Sprintf("prompt://%s", name)

			createLinkPart := ""
			if appPrompts[name] != "" {
				encodedPrompt := url.PathEscape(appPrompts[name])
				createLink := fmt.Sprintf("prompt://%s", encodedPrompt)

				shortCreateLink := createLink
				if len(shortCreateLink) > 28 {
					shortCreateLink = shortCreateLink[:25] + "..."
				}

				createLinkPart = fmt.Sprintf(`
                    <div class="link-item">
                        <span class="link-label">Create Link</span>
                        <div class="link-copy-wrapper">
                            <code class="link-code" title="%s">%s</code>
                            <button class="copy-btn" data-clipboard="%s" title="Copy Create Link">
                                <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                            </button>
                        </div>
                    </div>`, createLink, shortCreateLink, createLink)
			}

			publishBtnPart := ""
			if cfg.SharingToken != "" {
				publishBtnPart = fmt.Sprintf(`
                    <button class="tool-btn" onclick="publishApp('%s')" title="Publish App to Registry">
                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="17 8 12 3 7 8"></polyline><line x1="12" y1="3" x2="12" y2="15"></line></svg>
                    </button>`, name)
			}

			gridHTML += fmt.Sprintf(`
            <div class="card" id="card-%s">
                <div class="card-header">
                    <h3 class="card-title" id="title-%s">%s</h3>
                    <span class="card-badge">Local App</span>
                </div>
                <p class="card-desc" id="desc-%s">%s</p>
                
                <div class="card-toolbar">
                    <button class="tool-btn" onclick="toggleInlineEdit('%s')" title="Edit App">
                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9"></path><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z"></path></svg>
                    </button>
                    <button class="tool-btn" onclick="triggerInlineRename('%s')" title="Rename App">
                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path><path d="M18.5 2.5a2.12 2.12 0 0 1 3 3L12 15l-4 1 1-4Z"></path></svg>
                    </button>
                    %s
                    <button class="tool-btn delete" onclick="triggerInlineDelete('%s')" title="Delete App">
                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path><line x1="10" y1="11" x2="10" y2="17"></line><line x1="14" y1="11" x2="14" y2="17"></line></svg>
                    </button>
                </div>

                <div class="inline-edit-panel" id="edit-panel-%s" style="display: none;">
                    <textarea class="edit-textarea" id="edit-input-%s" placeholder="Describe edits... (e.g., Change accent colors, add feature x)"></textarea>
                    <button class="card-btn" style="margin-top: 8px; width: 100%%;" onclick="submitInlineEdit('%s')" id="edit-submit-%s">Update Application</button>
                    <div class="edit-loading" id="edit-loading-%s" style="display: none;">
                        <div class="spinner"></div> Updating application...
                    </div>
                </div>

                <div class="deep-links">
                    <div class="link-item">
                        <span class="link-label">Run Link</span>
                        <div class="link-copy-wrapper">
                            <code class="link-code">%s</code>
                            <button class="copy-btn" data-clipboard="%s" title="Copy Run Link">
                                <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                            </button>
                        </div>
                    </div>
                    %s
                </div>
                <div class="card-actions">
                    <a href="/apps/%s/" target="_blank" class="card-btn">Open Application</a>
                </div>
            </div>`, name, name, displayName, name, displayPrompt, name, name, publishBtnPart, name, name, name, name, name, name, runLink, runLink, createLinkPart, name)
		}
		gridHTML += `</div>`
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Promptyly Hub</title>
    <link href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #080c14;
            --panel-bg: rgba(15, 23, 42, 0.6);
            --border-color: rgba(255, 255, 255, 0.06);
            --border-hover: rgba(99, 102, 241, 0.3);
            --text-primary: #f8fafc;
            --text-secondary: #94a3b8;
            --text-muted: #64748b;
            --accent-color: #6366f1;
            --accent-hover: #4f46e5;
            --accent-glow: rgba(99, 102, 241, 0.25);
            --accent-grad: linear-gradient(135deg, #a5b4fc 0%%, #6366f1 100%%);
            --card-bg: rgba(15, 23, 42, 0.45);
            --success-color: #10b981;
            --error-color: #ef4444;
        }
        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }
        body {
            background-color: var(--bg-color);
            color: var(--text-primary);
            font-family: 'Plus Jakarta Sans', sans-serif;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            align-items: center;
            padding: 40px 20px;
        }
        .container {
            max-width: 1200px;
            width: 100%%;
            display: flex;
            flex-direction: column;
            gap: 32px;
        }
        header {
            text-align: center;
            display: flex;
            flex-direction: column;
            align-items: center;
            gap: 12px;
        }
        .brand-logo-wrapper {
            display: flex;
            justify-content: center;
            margin-bottom: 4px;
        }
        .brand-logo {
            width: 44px;
            height: 44px;
            background: var(--accent-grad);
            border-radius: 10px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: 800;
            font-size: 1.6rem;
            color: white;
            box-shadow: 0 0 25px var(--accent-glow);
        }
        h1 {
            font-size: 2.6rem;
            font-weight: 700;
            background: linear-gradient(135deg, #f8fafc 30%%, #94a3b8 100%%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            letter-spacing: -0.03em;
        }
        p.subtitle {
            color: var(--text-secondary);
            font-size: 1.05rem;
            max-width: 600px;
            line-height: 1.5;
        }
        
        /* Navigation Tabs */
        .nav-tabs {
            display: flex;
            gap: 12px;
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 8px;
            width: 100%%;
            margin-bottom: 10px;
        }
        .tab-btn {
            background: transparent;
            border: none;
            color: var(--text-secondary);
            padding: 10px 18px;
            font-family: inherit;
            font-size: 0.95rem;
            font-weight: 600;
            cursor: pointer;
            border-radius: 8px;
            transition: all 0.2s ease;
        }
        .tab-btn:hover {
            color: var(--text-primary);
            background: rgba(255, 255, 255, 0.03);
        }
        .tab-btn.active {
            color: white;
            background: var(--accent-grad);
            box-shadow: 0 0 10px var(--accent-glow);
        }
        .tab-panel {
            display: none;
            width: 100%%;
            flex-direction: column;
            gap: 32px;
        }
        .tab-panel.active {
            display: flex;
        }

        .gen-box {
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 16px;
            padding: 28px;
            width: 100%%;
            backdrop-filter: blur(20px);
        }
        .gen-box h2 {
            font-size: 1.4rem;
            font-weight: 700;
            margin-bottom: 6px;
            color: var(--text-primary);
        }
        .gen-subtitle {
            color: var(--text-secondary);
            font-size: 0.9rem;
            margin-bottom: 20px;
        }
        .gen-input-wrapper {
            display: flex;
            flex-direction: column;
            gap: 12px;
            width: 100%%;
        }
        @media(min-width: 768px) {
            .gen-input-wrapper {
                flex-direction: row;
                align-items: stretch;
            }
        }
        #gen-prompt, .form-textarea {
            flex-grow: 1;
            background: rgba(0, 0, 0, 0.2);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 14px;
            color: var(--text-primary);
            font-family: inherit;
            font-size: 0.95rem;
            resize: vertical;
            min-height: 54px;
            transition: border-color 0.2s;
        }
        #gen-prompt:focus, .form-textarea:focus, .form-control:focus {
            outline: none;
            border-color: var(--accent-color);
            box-shadow: 0 0 10px rgba(99, 102, 241, 0.15);
        }
        #btn-gen-submit, .btn-primary {
            background: var(--accent-grad);
            color: white;
            border: none;
            border-radius: 8px;
            padding: 12px 24px;
            font-weight: 600;
            font-size: 0.95rem;
            cursor: pointer;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.2s ease;
            white-space: nowrap;
        }
        #btn-gen-submit:hover, .btn-primary:hover {
            box-shadow: 0 0 15px var(--accent-glow);
            transform: translateY(-1px);
        }
        .gen-loading {
            display: flex;
            align-items: center;
            gap: 20px;
            padding: 20px;
            background: rgba(99, 102, 241, 0.05);
            border: 1px dashed rgba(99, 102, 241, 0.3);
            border-radius: 8px;
            margin-top: 15px;
        }
        .gen-loading-spinner {
            width: 36px;
            height: 36px;
            border: 3px solid rgba(99, 102, 241, 0.2);
            border-top-color: var(--accent-color);
            border-radius: 50%%;
            animation: spin 1s linear infinite;
            flex-shrink: 0;
        }
        .gen-loading-text {
            display: flex;
            flex-direction: column;
            gap: 4px;
        }
        .gen-loading-text strong {
            color: var(--text-primary);
            font-size: 0.95rem;
        }
        .gen-loading-text span {
            color: var(--text-secondary);
            font-size: 0.85rem;
        }
        .grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
            gap: 24px;
            width: 100%%;
        }
        .card {
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 16px;
            padding: 24px;
            backdrop-filter: blur(20px);
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            display: flex;
            flex-direction: column;
            justify-content: space-between;
            min-height: 280px;
        }
        .card:hover {
            transform: translateY(-4px);
            border-color: var(--border-hover);
            box-shadow: 0 12px 30px -10px var(--accent-glow);
        }
        .card-header {
            display: flex;
            justify-content: space-between;
            align-items: flex-start;
            margin-bottom: 12px;
            gap: 12px;
        }
        .card-title {
            font-size: 1.25rem;
            font-weight: 700;
            color: var(--text-primary);
            letter-spacing: -0.01em;
            line-height: 1.3;
        }
        .card-badge {
            background: rgba(99, 102, 241, 0.1);
            color: #a5b4fc;
            padding: 4px 8px;
            border-radius: 6px;
            font-size: 0.75rem;
            font-weight: 600;
            white-space: nowrap;
        }
        .card-desc {
            color: var(--text-secondary);
            font-size: 0.9rem;
            line-height: 1.5;
            margin: 0 0 16px 0;
            flex-grow: 1;
        }
        .card-toolbar {
            display: flex;
            gap: 8px;
            margin-bottom: 16px;
            background: rgba(0, 0, 0, 0.15);
            padding: 6px 12px;
            border-radius: 8px;
            border: 1px solid rgba(255, 255, 255, 0.02);
            justify-content: flex-end;
        }
        .tool-btn {
            background: transparent;
            border: none;
            color: var(--text-secondary);
            cursor: pointer;
            padding: 6px;
            border-radius: 4px;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.2s ease;
        }
        .tool-btn:hover {
            color: var(--text-primary);
            background: rgba(255, 255, 255, 0.05);
        }
        .tool-btn.delete:hover {
            color: var(--error-color);
            background: rgba(239, 68, 68, 0.08);
        }
        .inline-edit-panel {
            background: rgba(0, 0, 0, 0.25);
            border: 1px solid rgba(255, 255, 255, 0.04);
            border-radius: 10px;
            padding: 12px;
            margin-bottom: 16px;
            display: flex;
            flex-direction: column;
            gap: 10px;
        }
        .edit-textarea {
            width: 100%%;
            background: rgba(0, 0, 0, 0.2);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            padding: 8px 10px;
            color: var(--text-primary);
            font-family: inherit;
            font-size: 0.85rem;
            resize: vertical;
            min-height: 60px;
        }
        .edit-textarea:focus {
            outline: none;
            border-color: var(--accent-color);
        }
        .edit-loading {
            display: flex;
            align-items: center;
            gap: 10px;
            font-size: 0.85rem;
            color: var(--text-secondary);
            margin-top: 4px;
        }
        .spinner {
            width: 20px;
            height: 20px;
            border: 2px solid rgba(99, 102, 241, 0.2);
            border-top-color: var(--accent-color);
            border-radius: 50%%;
            animation: spin 1s linear infinite;
        }
        .deep-links {
            background: rgba(0, 0, 0, 0.2);
            border: 1px solid rgba(255, 255, 255, 0.03);
            border-radius: 10px;
            padding: 12px;
            margin-bottom: 20px;
            display: flex;
            flex-direction: column;
            gap: 10px;
        }
        .link-item {
            display: flex;
            flex-direction: column;
            gap: 4px;
        }
        .link-label {
            font-size: 0.7rem;
            font-weight: 600;
            color: var(--text-muted);
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        .link-copy-wrapper {
            display: flex;
            align-items: center;
            justify-content: space-between;
            background: rgba(255, 255, 255, 0.02);
            border: 1px solid rgba(255, 255, 255, 0.04);
            border-radius: 6px;
            padding: 6px 10px;
            gap: 8px;
        }
        .link-code {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.78rem;
            color: #a5b4fc;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            user-select: all;
        }
        .copy-btn {
            background: transparent;
            border: none;
            color: var(--text-secondary);
            cursor: pointer;
            padding: 4px;
            border-radius: 4px;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.2s ease;
            flex-shrink: 0;
        }
        .copy-btn:hover {
            color: var(--text-primary);
            background: rgba(255, 255, 255, 0.05);
        }
        .copy-btn.copied {
            color: var(--success-color);
        }
        .card-actions {
            display: flex;
            gap: 12px;
        }
        .card-btn {
            flex-grow: 1;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            padding: 10px 16px;
            background-color: var(--accent-color);
            color: white;
            text-decoration: none;
            border-radius: 8px;
            font-weight: 600;
            font-size: 0.9rem;
            transition: all 0.2s ease;
            border: 1px solid transparent;
            cursor: pointer;
        }
        .card-btn:hover {
            background-color: var(--accent-hover);
            box-shadow: 0 0 12px rgba(99, 102, 241, 0.4);
        }
        .empty-state {
            text-align: center;
            padding: 60px 40px;
            border: 1px dashed var(--border-color);
            border-radius: 16px;
            color: var(--text-secondary);
            background: rgba(15, 23, 42, 0.2);
            max-width: 500px;
            margin: 0 auto;
            width: 100%%;
        }
        .empty-state h3 {
            margin-top: 0;
            color: var(--text-primary);
            margin-bottom: 8px;
        }
        @keyframes spin {
            to { transform: rotate(360deg); }
        }

        /* Settings CSS */
        .settings-grid {
            display: flex;
            flex-direction: column;
            gap: 24px;
            max-width: 800px;
            margin: 0 auto;
            width: 100%%;
        }
        .settings-section {
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 16px;
            padding: 24px;
        }
        .settings-section-title {
            font-size: 1.25rem;
            margin-bottom: 16px;
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 8px;
            color: white;
        }
        .form-group {
            display: flex;
            flex-direction: column;
            gap: 8px;
            margin-bottom: 16px;
        }
        .form-label {
            font-size: 0.85rem;
            font-weight: 600;
            color: var(--text-secondary);
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        .form-control {
            background: rgba(0, 0, 0, 0.2);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            padding: 10px 12px;
            color: var(--text-primary);
            font-family: inherit;
            font-size: 0.9rem;
            transition: all 0.2s;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <div class="brand-logo-wrapper">
                <div class="brand-logo">P</div>
            </div>
            <h1>Promptyly Hub</h1>
            <p class="subtitle">Your locally generated, git-backed web applications</p>
        </header>

        <div class="nav-tabs">
            <button class="tab-btn active" id="tab-local" onclick="switchTab('local')">Local Workspace</button>
            <button class="tab-btn" id="tab-remote" onclick="switchTab('remote')">Remote Registry</button>
            <button class="tab-btn" id="tab-settings" onclick="switchTab('settings')">Settings & Config</button>
        </div>

        <!-- PANEL 1: Local workspace dashboard -->
        <div id="panel-local" class="tab-panel active">
            <div class="gen-box">
                <h2>Create New Application</h2>
                <p class="gen-subtitle">Enter a prompt describing the single-page application you want to build</p>
                <div class="gen-input-wrapper">
                    <textarea id="gen-prompt" placeholder="E.g., A sleek Pomodoro Timer with dark mode, ambient sound selector, and statistics chart..."></textarea>
                    <button id="btn-gen-submit" onclick="submitCreateApp()">
                        <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; vertical-align: middle;"><line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line></svg>
                        Generate App
                    </button>
                </div>
                <div class="gen-loading" id="gen-loading" style="display: none;">
                    <div class="gen-loading-spinner"></div>
                    <div class="gen-loading-text">
                        <strong>Generating Web Application...</strong>
                        <span>Promptyly is writing your files and initializing git history. This may take 15-30 seconds.</span>
                    </div>
                </div>
            </div>
            
            %s
        </div>

        <!-- PANEL 2: Remote App Search -->
        <div id="panel-remote" class="tab-panel">
            <div class="gen-box">
                <h2>Remote Sharing Registry</h2>
                <p class="gen-subtitle">Browse, search, and download applications shared by other developers and machines</p>
                <div class="gen-input-wrapper">
                    <input type="text" id="remote-q" placeholder="Search remote registry by name, prompt text, creator..." class="form-control" style="flex-grow: 1; height: 54px; margin-bottom: 0;">
                    <button class="btn-primary" onclick="searchRemote()">
                        <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; vertical-align: middle;"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>
                        Search Registry
                    </button>
                </div>
            </div>

            <div class="grid" id="remote-grid">
                <div class="empty-state" style="grid-column: 1/-1;">
                    <h3>Registry Search</h3>
                    <p>Enter search query above to load remote sharing registry apps.</p>
                </div>
            </div>
        </div>

        <!-- PANEL 3: Config settings panel -->
        <div id="panel-settings" class="tab-panel">
            <div class="settings-grid">
                <!-- AI configurations -->
                <div class="settings-section">
                    <h3 class="settings-section-title">AI API Integration</h3>
                    <div class="form-group">
                        <label class="form-label">Default LLM Provider</label>
                        <select class="form-control" id="set-provider">
                            <option value="gemini">Gemini (Google)</option>
                            <option value="claude">Claude (Anthropic)</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Gemini API Key</label>
                        <input type="password" class="form-control" id="set-gemini-key" placeholder="Enter Gemini Key">
                    </div>
                    <div class="form-group">
                        <label class="form-label">Gemini Model</label>
                        <input type="text" class="form-control" id="set-gemini-model" placeholder="gemini-1.5-flash">
                    </div>
                    <div class="form-group">
                        <label class="form-label">Claude API Key</label>
                        <input type="password" class="form-control" id="set-claude-key" placeholder="Enter Claude Key">
                    </div>
                    <div class="form-group">
                        <label class="form-label">Claude Model</label>
                        <input type="text" class="form-control" id="set-claude-model" placeholder="claude-3-5-sonnet-20240620">
                    </div>
                </div>

                <!-- Sharing settings -->
                <div class="settings-section">
                    <h3 class="settings-section-title">Sharing Registry Server</h3>
                    <div class="form-group">
                        <label class="form-label">Registry Server URL</label>
                        <input type="text" class="form-control" id="set-sharing-url" placeholder="http://localhost:6072">
                    </div>
                    <div class="form-group">
                        <label class="form-label">Registry API Token</label>
                        <input type="password" class="form-control" id="set-sharing-token" placeholder="Enter API Token">
                    </div>
                    <div class="form-group">
                        <label class="form-label">Local Apps Folder</label>
                        <input type="text" class="form-control" id="set-apps-dir" placeholder="~/promptyly-apps">
                    </div>
                    <div class="form-group" style="flex-direction: row; align-items: center; gap: 8px;">
                        <input type="checkbox" id="set-check-remote" style="width: auto;">
                        <label class="form-label" style="text-transform: none; margin-bottom: 0;">Check Remote Registry Before Generating New Apps</label>
                    </div>
                </div>

                <button class="btn-primary" onclick="saveSettings()" style="padding: 14px; font-size: 1rem; font-weight: 700;">Save All Configuration Settings</button>
            </div>
        </div>
    </div>

    <script>
    const API_TOKEN = "%s";
    const LOCAL_APPS = %s;
    let activeConfig = null;

    function slugify(s) {
        s = s.toLowerCase();
        let res = '';
        for (let i = 0; i < s.length; i++) {
            const char = s[i];
            if ((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')) {
                res += char;
            } else if (char === ' ' || char === '-' || char === '_') {
                if (res.length > 0 && res[res.length - 1] !== '-') {
                    res += '-';
                }
            }
        }
        res = res.replace(/-+$/, '');
        if (res.length > 30) {
            res = res.substring(0, 30).replace(/-+$/, '');
        }
        return res || 'app';
    }

    function switchTab(tabId) {
        document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
        document.querySelectorAll('.tab-panel').forEach(panel => panel.classList.remove('active'));
        
        document.getElementById('tab-' + tabId).classList.add('active');
        document.getElementById('panel-' + tabId).classList.add('active');

        if (tabId === 'settings') {
            loadSettings();
        }
    }

    async function loadSettings() {
        try {
            const res = await fetch('/api/config', {
                headers: { 'X-Promptyly-Token': API_TOKEN }
            });
            if (!res.ok) throw new Error('Daemon config error');
            activeConfig = await res.json();

            document.getElementById('set-provider').value = activeConfig.default_provider || 'gemini';
            document.getElementById('set-apps-dir').value = activeConfig.apps_dir || '';
            document.getElementById('set-sharing-url').value = activeConfig.sharing_server_url || '';
            document.getElementById('set-sharing-token').value = activeConfig.sharing_token || '';
            document.getElementById('set-check-remote').checked = !!activeConfig.check_remote_first;
            
            const gemini = activeConfig.providers.gemini || {};
            document.getElementById('set-gemini-key').value = gemini.api_key || '';
            document.getElementById('set-gemini-model').value = gemini.model || '';

            const claude = activeConfig.providers.claude || {};
            document.getElementById('set-claude-key').value = claude.api_key || '';
            document.getElementById('set-claude-model').value = claude.model || '';
        } catch (err) {
            console.error('Failed to load daemon config: ', err);
        }
    }

    async function saveSettings() {
        if (!activeConfig) return;

        activeConfig.default_provider = document.getElementById('set-provider').value;
        activeConfig.apps_dir = document.getElementById('set-apps-dir').value.trim();
        activeConfig.sharing_server_url = document.getElementById('set-sharing-url').value.trim();
        activeConfig.sharing_token = document.getElementById('set-sharing-token').value.trim();
        activeConfig.check_remote_first = document.getElementById('set-check-remote').checked;

        activeConfig.providers.gemini = {
            api_key: document.getElementById('set-gemini-key').value.trim(),
            model: document.getElementById('set-gemini-model').value.trim()
        };

        activeConfig.providers.claude = {
            api_key: document.getElementById('set-claude-key').value.trim(),
            model: document.getElementById('set-claude-model').value.trim()
        };

        try {
            const res = await fetch('/api/config', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Promptyly-Token': API_TOKEN
                },
                body: JSON.stringify(activeConfig)
            });
            if (res.ok) {
                alert('Configuration settings updated successfully!');
                location.reload();
            } else {
                alert('Failed to save settings: ' + await res.text());
            }
        } catch (err) {
            alert('Error saving config: ' + err.message);
        }
    }

    async function searchRemote() {
        if (!activeConfig) {
            await loadSettings();
        }
        const q = document.getElementById('remote-q').value.trim();
        const serverUrl = activeConfig.sharing_server_url || 'http://localhost:6072';
        const grid = document.getElementById('remote-grid');
        grid.innerHTML = '<div class="empty-state" style="grid-column: 1/-1;"><div class="spinner" style="margin: 0 auto 12px auto;"></div>Querying Remote Registry...</div>';

        try {
            const res = await fetch(serverUrl + '/api/apps/search?q=' + encodeURIComponent(q));
            if (!res.ok) throw new Error('Remote search failed');
            const apps = await res.json();

            if (apps.length === 0) {
                grid.innerHTML = '<div class="empty-state" style="grid-column: 1/-1;"><h3>No applications found</h3><p>Try searching another keyword.</p></div>';
                return;
            }

            let html = '';
            apps.forEach(app => {
                const viewsText = app.views === 1 ? '1 view' : app.views + ' views';
                const downloadsText = app.downloads === 1 ? '1 download' : app.downloads + ' downloads';
                const slugName = slugify(app.name);
                const isLocal = LOCAL_APPS.includes(app.name) || LOCAL_APPS.includes(slugName);

                html += '<div class="card" id="remote-card-' + app.id + '" data-app-name="' + escapeHTML(app.name) + '">' +
'                    <div class="card-header">' +
'                        <h3 class="card-title">' + escapeHTML(app.name) + '</h3>' +
'                        <span class="card-badge">by ' + escapeHTML(app.username) + '</span>' +
'                    </div>' +
'                    <p class="card-desc">' + escapeHTML(app.prompt) + '</p>' +
'                    ' +
'                    <div class="deep-links">' +
'                        <div class="link-item">' +
'                            <span class="link-label">Stats & Date</span>' +
'                            <div class="link-copy-wrapper">' +
'                                <code class="link-code">' + viewsText + ' &bull; ' + downloadsText + '</code>' +
'                            </div>' +
'                        </div>' +
'                    </div>' +
'                    ' +
'                    <div class="card-actions">' +
'                        <a href="' + serverUrl + '/apps/' + app.id + '/" target="_blank" class="card-btn" style="background: rgba(255,255,255,0.05); color: var(--text-primary); border: 1px solid var(--border-color); flex-grow: 0; padding: 10px 14px;">Live App</a>' +
'                        <button onclick="toggleRemoteInlineEdit(\'' + app.id + '\')" class="card-btn" style="background: rgba(99,102,241,0.15); color: #a5b4fc; border: 1px solid rgba(99,102,241,0.3); flex-grow: 0; padding: 10px 14px;" title="Edit App">Edit</button>' +
'                        <button onclick="installRemoteApp(\'' + app.id + '\', this)" class="card-btn">' + (isLocal ? 'Re-install' : 'Install Locally') + '</button>' +
'                    </div>' +
'                    ' +
'                    <div class="inline-edit-panel" id="remote-edit-panel-' + app.id + '" style="display: none; flex-direction: column; margin-top: 15px; border-top: 1px solid var(--border-color); padding-top: 15px;">' +
'                        <textarea class="edit-textarea" id="remote-edit-input-' + app.id + '" placeholder="Describe edits... (e.g., Change accent colors, add feature x)"></textarea>' +
'                        <button class="card-btn" style="margin-top: 8px; width: 100%;" onclick="submitRemoteInlineEdit(\'' + app.id + '\')" id="remote-edit-submit-' + app.id + '">Update Application</button>' +
'                        <div class="edit-loading" id="remote-edit-loading-' + app.id + '" style="display: none;">' +
'                            <div class="spinner"></div> <span id="remote-edit-loading-text-' + app.id + '">Updating application...</span>' +
'                        </div>' +
'                    </div>' +
'                </div>';
            });
            grid.innerHTML = html;
        } catch (err) {
            grid.innerHTML = '<div class="empty-state" style="grid-column: 1/-1; border-color: var(--error-color); color: var(--error-color);"><h3>Registry Connection Failed</h3><p>Ensure the sharing server at <code>' + serverUrl + '</code> is running and configured correctly in settings.</p></div>';
        }
    }

    function toggleRemoteInlineEdit(id) {
        const panel = document.getElementById('remote-edit-panel-' + id);
        if (panel) {
            panel.style.display = panel.style.display === 'none' ? 'flex' : 'none';
        }
    }

    async function submitRemoteInlineEdit(id) {
        const input = document.getElementById('remote-edit-input-' + id);
        const button = document.getElementById('remote-edit-submit-' + id);
        const loading = document.getElementById('remote-edit-loading-' + id);
        const loadingText = document.getElementById('remote-edit-loading-text-' + id);
        const card = document.getElementById('remote-card-' + id);

        if (!input || !input.value.trim() || !card) return;

        const appName = card.getAttribute('data-app-name');
        const promptVal = input.value.trim();
        input.disabled = true;
        button.disabled = true;
        loading.style.display = 'flex';

        const slugName = slugify(appName);
        const isLocal = LOCAL_APPS.includes(appName) || LOCAL_APPS.includes(slugName);

        try {
            let localName = appName;
            if (!isLocal) {
                loadingText.textContent = 'Downloading base application...';
                const dlRes = await fetch('/api/apps/download', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-Promptyly-Token': API_TOKEN
                    },
                    body: JSON.stringify({ appId: id })
                });
                if (!dlRes.ok) {
                    throw new Error('Download failed: ' + await dlRes.text());
                }
                const dlData = await dlRes.json();
                localName = dlData.appName;
            }

            loadingText.textContent = 'Updating application...';
            const editRes = await fetch('/api/apps/edit', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Promptyly-Token': API_TOKEN
                },
                body: JSON.stringify({ name: localName, prompt: promptVal })
            });

            if (editRes.ok) {
                alert('Application updated successfully!');
                window.location.reload();
            } else {
                const txt = await editRes.text();
                alert('Failed to update: ' + txt);
                input.disabled = false;
                button.disabled = false;
                loading.style.display = 'none';
            }
        } catch (err) {
            alert('Error updating: ' + err.message);
            input.disabled = false;
            button.disabled = false;
            loading.style.display = 'none';
        }
    }

    async function installRemoteApp(appId, button) {
        const originalText = button.textContent;
        button.disabled = true;
        button.textContent = 'Installing...';

        try {
            const res = await fetch('/api/apps/download', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Promptyly-Token': API_TOKEN
                },
                body: JSON.stringify({ appId: appId })
            });

            if (res.ok) {
                const data = await res.json();
                alert('Success! Application "' + data.appName + '" has been downloaded and installed locally!');
                window.location.reload();
            } else {
                alert('Installation failed: ' + await res.text());
                button.disabled = false;
                button.textContent = originalText;
            }
        } catch (err) {
            alert('Installation error: ' + err.message);
            button.disabled = false;
            button.textContent = originalText;
        }
    }

    function escapeHTML(str) {
        if (!str) return '';
        return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#039;');
    }

    // Load initial config on page boot
    document.addEventListener('DOMContentLoaded', () => {
        loadSettings();
    });

    function toggleInlineEdit(name) {
        const panel = document.getElementById('edit-panel-' + name);
        if (panel) {
            panel.style.display = panel.style.display === 'none' ? 'flex' : 'none';
        }
    }

    async function submitInlineEdit(name) {
        const input = document.getElementById('edit-input-' + name);
        const button = document.getElementById('edit-submit-' + name);
        const loading = document.getElementById('edit-loading-' + name);

        if (!input || !input.value.trim()) return;

        input.disabled = true;
        button.disabled = true;
        loading.style.display = 'flex';

        try {
            const res = await fetch('/api/apps/edit', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'X-Promptyly-Token': API_TOKEN
                },
                body: JSON.stringify({ name: name, prompt: input.value.trim() })
            });
            if (res.ok) {
                alert('Application updated successfully!');
                window.location.reload();
            } else {
                const txt = await res.text();
                alert('Failed to update: ' + txt);
                input.disabled = false;
                button.disabled = false;
                loading.style.display = 'none';
            }
        } catch (err) {
            alert('Error updating: ' + err.message);
            input.disabled = false;
            button.disabled = false;
            loading.style.display = 'none';
        }
    }

    async function triggerInlineRename(name) {
        const titleEl = document.getElementById('title-' + name);
        const currentName = titleEl ? titleEl.textContent : name;
        const newName = prompt('Enter new name for "' + currentName + '":', currentName);
        if (!newName || newName.trim() === currentName) return;

        try {
            const res = await fetch('/api/apps/rename', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'X-Promptyly-Token': API_TOKEN
                },
                body: JSON.stringify({ oldName: name, newName: newName.trim() })
            });
            if (res.ok) {
                alert('Application renamed successfully!');
                window.location.reload();
            } else {
                const txt = await res.text();
                alert('Failed to rename: ' + txt);
            }
        } catch (err) {
            alert('Error renaming: ' + err.message);
        }
    }

    async function triggerInlineDelete(name) {
        const titleEl = document.getElementById('title-' + name);
        const currentName = titleEl ? titleEl.textContent : name;
        if (!confirm('Are you sure you want to remove the application "' + currentName + '"?')) return;
        const deleteFolder = confirm('Do you also want to permanently delete the files on disk for this app?');

        try {
            const res = await fetch('/api/apps/delete', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'X-Promptyly-Token': API_TOKEN
                },
                body: JSON.stringify({ name: name, deleteFolder: deleteFolder })
            });
            if (res.ok) {
                alert('Application deleted successfully!');
                window.location.reload();
            } else {
                const txt = await res.text();
                alert('Failed to delete: ' + txt);
            }
        } catch (err) {
            alert('Error deleting: ' + err.message);
        }
    }

    async function publishApp(name) {
        const desc = prompt("Enter an optional description for the sharing registry:", "");
        if (desc === null) return;

        const card = document.getElementById('card-' + name);
        const btn = card.querySelector('button[title="Publish App to Registry"]');
        if (!btn) {
            alert('Publishing is not configured. Please configure your Registry API Token in Settings.');
            return;
        }
        const originalHTML = btn.innerHTML;
        btn.disabled = true;
        btn.innerHTML = '<div class="spinner" style="width: 14px; height: 14px; margin: 0 auto;"></div>';

        try {
            const res = await fetch('/api/apps/publish', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Promptyly-Token': API_TOKEN
                },
                body: JSON.stringify({ name: name, description: desc.trim() })
            });

            if (res.ok) {
                const data = await res.json();
                alert('Success! Application successfully published to the registry!\nLive URL: ' + data.liveUrl + '\nDetail Page: ' + data.detailUrl);
            } else {
                const txt = await res.text();
                alert('Publish failed: ' + txt);
            }
        } catch (err) {
            alert('Publish error: ' + err.message);
        } finally {
            btn.disabled = false;
            btn.innerHTML = originalHTML;
        }
    }

    async function submitCreateApp() {
        const promptInput = document.getElementById('gen-prompt');
        const submitBtn = document.getElementById('btn-gen-submit');
        const loading = document.getElementById('gen-loading');

        if (!promptInput || !promptInput.value.trim()) return;

        promptInput.disabled = true;
        submitBtn.disabled = true;
        loading.style.display = 'flex';

        try {
            const res = await fetch('/api/apps/create', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'X-Promptyly-Token': API_TOKEN
                },
                body: JSON.stringify({ prompt: promptInput.value.trim() })
            });
            if (res.ok) {
                const data = await res.json();
                alert('Application "' + data.appName + '" created successfully!');
                window.location.reload();
            } else {
                const txt = await res.text();
                alert('Generation failed: ' + txt);
                promptInput.disabled = false;
                submitBtn.disabled = false;
                loading.style.display = 'none';
            }
        } catch (err) {
            alert('Error generating: ' + err.message);
            promptInput.disabled = false;
            submitBtn.disabled = false;
            loading.style.display = 'none';
        }
    }

    document.querySelectorAll('.copy-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.stopPropagation();
            const text = btn.getAttribute('data-clipboard');
            if (!text) return;
            navigator.clipboard.writeText(text).then(() => {
                const originalHTML = btn.innerHTML;
                btn.classList.add('copied');
                btn.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>';
                setTimeout(() => {
                    btn.classList.remove('copied');
                    btn.innerHTML = originalHTML;
                }, 2000);
            }).catch(err => {
                console.error('Failed to copy text: ', err);
            });
        });
    });
    </script>
</body>
</html>`, gridHTML, apiToken, string(localAppsJSON))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

func appsHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/apps/") {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(strings.TrimPrefix(path, "/apps/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "App name not specified", http.StatusBadRequest)
		return
	}
	appName := parts[0]

	// Redirect if trailing slash is missing from directory path root
	if len(parts) == 1 && !strings.HasSuffix(path, "/") {
		http.Redirect(w, r, path+"/", http.StatusMovedPermanently)
		return
	}

	cfg, err := getCachedConfig()
	if err != nil {
		http.Error(w, "Failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	appDir, ok := cfg.Apps[appName]
	if !ok {
		http.Error(w, fmt.Sprintf("App '%s' not found in registry", appName), http.StatusNotFound)
		return
	}

	relPath := "/" + strings.Join(parts[1:], "/")

	// Dynamic DB API Endpoint
	if relPath == "/_promptyly/api/db" || relPath == "/_promptyly/api/db/" {
		handleDb(appDir)(w, r)
		return
	}

	// SSE Connection for hot reload
	if relPath == "/_promptyly/events" {
		eventsHandler(appName, w, r)
		return
	}

	// Trigger reload webhook (POST from other terminals/processes)
	if relPath == "/_promptyly/reload" {
		if r.Method == "POST" {
			NotifyReload(appName)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"reloaded"}`))
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Serve HTML and inject Live Reload references
	if relPath == "/" || relPath == "/index.html" || strings.HasSuffix(relPath, ".html") {
		filePath := filepath.Join(appDir, "index.html")
		if relPath != "/" && relPath != "/index.html" {
			filePath = filepath.Join(appDir, filepath.Clean(relPath))
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")

		ssePath := fmt.Sprintf("/apps/%s/_promptyly/events", appName)
		_, _ = w.Write([]byte(injectLiveReload(string(data), ssePath)))
		return
	}

	fileServerPath := filepath.Join(appDir, filepath.Clean(relPath))
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	http.ServeFile(w, r, fileServerPath)
}

func initToken() {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return
	}
	tokenPath := filepath.Join(configDir, ".token")
	
	// Read existing token if it's there
	if data, err := os.ReadFile(tokenPath); err == nil {
		apiToken = strings.TrimSpace(string(data))
		if apiToken != "" {
			return
		}
	}

	// Generate new token if not exists or empty
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		apiToken = fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	} else {
		apiToken = hex.EncodeToString(tokenBytes)
	}

	_ = os.WriteFile(tokenPath, []byte(apiToken), 0600)
}

func isAuthorized(r *http.Request) bool {
	if apiToken == "" {
		// Attempt to load token if not yet loaded in this execution instance
		initToken()
		if apiToken == "" {
			return true // Fallback to allowing if config directory is inaccessible
		}
	}
	return r.Header.Get("X-Promptyly-Token") == apiToken
}

func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if !isAuthorized(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// StartDevServer launches a local static server on the specified port.
// If the port is occupied, it returns nil error (assuming it's already running in the background/another process).
func StartDevServer(defaultPort int) (int, error) {
	initToken()

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", defaultPort))
	if err != nil {
		// Port already bound, assuming server is running in another terminal
		return defaultPort, nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", homeHandler)
	mux.HandleFunc("/apps/", appsHandler)

	// Desktop API routes
	mux.HandleFunc("/api/apps", withAuth(apiAppsHandler))
	mux.HandleFunc("/api/apps/create", withAuth(apiCreateAppHandler))
	mux.HandleFunc("/api/apps/edit", withAuth(apiEditAppHandler))
	mux.HandleFunc("/api/apps/rename", withAuth(apiRenameAppHandler))
	mux.HandleFunc("/api/apps/link", withAuth(apiLinkAppHandler))
	mux.HandleFunc("/api/apps/delete", withAuth(apiDeleteAppHandler))
	mux.HandleFunc("/api/apps/export", withAuth(apiExportAppHandler))
	mux.HandleFunc("/api/apps/update-metadata", withAuth(apiUpdateMetadataHandler))
	mux.HandleFunc("/api/apps/download", withAuth(apiDownloadAppHandler))
	mux.HandleFunc("/api/apps/publish", withAuth(apiPublishAppHandler))
	mux.HandleFunc("/api/apps/import", withAuth(apiImportAppHandler))
	mux.HandleFunc("/api/apps/search", withAuth(apiSearchAppsHandler))
	mux.HandleFunc("/api/protocol/register", withAuth(apiProtocolRegisterHandler))
	mux.HandleFunc("/api/protocol/unregister", withAuth(apiProtocolUnregisterHandler))
	mux.HandleFunc("/api/config", withAuth(apiConfigHandler))

	go func() {
		_ = http.Serve(ln, mux)
	}()

	return defaultPort, nil
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Promptyly-Token")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
}

func apiAppsHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg, err := getCachedConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	appPrompts := make(map[string]string)
	for pr, name := range cfg.Prompts {
		appPrompts[name] = pr
	}

	type AppInfo struct {
		Path   string `json:"path"`
		Prompt string `json:"prompt"`
	}
	res := make(map[string]AppInfo)
	for name, path := range cfg.Apps {
		res[name] = AppInfo{
			Path:   path,
			Prompt: appPrompts[name],
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func apiCreateAppHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if CreateAppCallback == nil {
		http.Error(w, "Create app callback not configured", http.StatusInternalServerError)
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	appName, appPath, err := CreateAppCallback(req.Prompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"appName": appName,
		"appPath": appPath,
	})
}

func apiEditAppHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if EditAppCallback == nil {
		http.Error(w, "Edit app callback not configured", http.StatusInternalServerError)
		return
	}

	var req struct {
		Name   string `json:"name"`
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := EditAppCallback(req.Name, req.Prompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	NotifyReload(req.Name)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func apiRenameAppHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if RenameAppCallback == nil {
		http.Error(w, "Rename app callback not configured", http.StatusInternalServerError)
		return
	}

	var req struct {
		OldName string `json:"oldName"`
		NewName string `json:"newName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slugName, err := RenameAppCallback(req.OldName, req.NewName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"appName": slugName,
	})
}

func apiLinkAppHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if LinkAppCallback == nil {
		http.Error(w, "Link app callback not configured", http.StatusInternalServerError)
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	appName, err := LinkAppCallback(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"appName": appName,
	})
}

func apiDeleteAppHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if DeleteAppCallback == nil {
		http.Error(w, "Delete app callback not configured", http.StatusInternalServerError)
		return
	}

	var req struct {
		Name         string `json:"name"`
		DeleteFolder bool   `json:"deleteFolder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := DeleteAppCallback(req.Name, req.DeleteFolder)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func apiConfigHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == "GET" {
		cfg, err := getCachedConfig()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cfg)
		return
	}

	if r.Method == "POST" {
		var loadedConfig config.Config
		if err := json.NewDecoder(r.Body).Decode(&loadedConfig); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.SaveConfig(&loadedConfig); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cachedConfigMu.Lock()
		cachedConfig = &loadedConfig
		cachedConfigMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func apiExportAppHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ExportAppCallback == nil {
		http.Error(w, "Export app callback not configured", http.StatusInternalServerError)
		return
	}

	var req struct {
		Name    string `json:"name"`
		ZipPath string `json:"zipPath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := ExportAppCallback(req.Name, req.ZipPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func apiUpdateMetadataHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if UpdateMetadataCallback == nil {
		http.Error(w, "Update metadata callback not configured", http.StatusInternalServerError)
		return
	}

	var req struct {
		Name      string `json:"name"`
		NewName   string `json:"newName"`
		NewPrompt string `json:"newPrompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slugName, err := UpdateMetadataCallback(req.Name, req.NewName, req.NewPrompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"appName": slugName,
	})
}

func apiDownloadAppHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ImportAppCallback == nil {
		http.Error(w, "Import app callback not configured", http.StatusInternalServerError)
		return
	}

	var req struct {
		AppID string `json:"appId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := reloadCachedConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	serverURL := cfg.SharingServerURL
	if serverURL == "" {
		serverURL = "http://localhost:6072"
	}

	u := fmt.Sprintf("%s/api/apps/download/%s", strings.TrimSuffix(serverURL, "/"), req.AppID)
	resp, err := http.Get(u)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to contact sharing server: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("Remote download failed (status %d): %s", resp.StatusCode, string(respBody)), http.StatusInternalServerError)
		return
	}

	tempZip := filepath.Join(os.TempDir(), fmt.Sprintf("promptyly-daemon-dl-%s.zip", req.AppID))
	out, err := os.Create(tempZip)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp zip: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempZip)

	_, err = io.Copy(out, resp.Body)
	_ = out.Close()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to write temp zip: %v", err), http.StatusInternalServerError)
		return
	}

	importedName, err := ImportAppCallback(tempZip)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to import app: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"appName": importedName,
	})
}

func apiPublishAppHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ExportAppCallback == nil {
		http.Error(w, "Export app callback not configured", http.StatusInternalServerError)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := reloadCachedConfig()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	appDir, ok := cfg.Apps[req.Name]
	if !ok {
		http.Error(w, fmt.Sprintf("App '%s' not found locally in your registry", req.Name), http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("App directory '%s' does not exist", appDir), http.StatusBadRequest)
		return
	}

	promptText := ""
	for pr, name := range cfg.Prompts {
		if name == req.Name {
			promptText = pr
			break
		}
	}
	if promptText == "" {
		promptText = "Generated web application."
	}

	tempZip := filepath.Join(os.TempDir(), fmt.Sprintf("promptyly-daemon-pub-%s.zip", req.Name))
	err = ExportAppCallback(req.Name, tempZip)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to package app: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempZip)

	serverURL := cfg.SharingServerURL
	if serverURL == "" {
		serverURL = "http://localhost:6072"
	}
	token := cfg.SharingToken
	if token == "" {
		http.Error(w, "Sharing registry API token not configured. Please sign in or set the token in Settings first.", http.StatusBadRequest)
		return
	}

	file, err := os.Open(tempZip)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open packaged zip: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(tempZip))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create form file: %v", err), http.StatusInternalServerError)
		return
	}
	_, err = io.Copy(part, file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to copy zip: %v", err), http.StatusInternalServerError)
		return
	}

	_ = writer.WriteField("name", req.Name)
	_ = writer.WriteField("prompt", promptText)
	_ = writer.WriteField("description", req.Description)
	_ = writer.Close()

	u := fmt.Sprintf("%s/api/apps/upload", strings.TrimSuffix(serverURL, "/"))
	uploadReq, err := http.NewRequest("POST", u, body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(uploadReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to contact sharing server: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Sharing server error (%d): %s", resp.StatusCode, string(respBody)), http.StatusInternalServerError)
		return
	}

	var res struct {
		Success bool   `json:"success"`
		AppID   string `json:"app_id"`
		URL     string `json:"url"`
	}
	if err := json.Unmarshal(respBody, &res); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse sharing server response: %v", err), http.StatusInternalServerError)
		return
	}

	cleanURL := strings.TrimSuffix(serverURL, "/") + res.URL
	detailURL := fmt.Sprintf("%s/app/%s", strings.TrimSuffix(serverURL, "/"), res.AppID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"appId":     res.AppID,
		"liveUrl":   cleanURL,
		"detailUrl": detailURL,
	})
}

func apiImportAppHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if ImportAppCallback == nil {
		http.Error(w, "Import app callback not configured", http.StatusInternalServerError)
		return
	}
	var req struct {
		ZipPath string `json:"zipPath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	appName, err := ImportAppCallback(req.ZipPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"appName": appName,
	})
}

func apiSearchAppsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query().Get("q")
	cfg, err := getCachedConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	serverURL := cfg.SharingServerURL
	if serverURL == "" {
		serverURL = "http://localhost:6072"
	}
	u := fmt.Sprintf("%s/api/apps/search?q=%s", strings.TrimSuffix(serverURL, "/"), url.QueryEscape(q))
	resp, err := http.Get(u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func apiProtocolRegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	err := urlscheme.Register()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func apiProtocolUnregisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	err := urlscheme.Unregister()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}



