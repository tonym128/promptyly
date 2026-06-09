package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"promptyly/config"
	"strings"
	"sync"
)

var (
	clients   = make(map[string]map[chan string]bool) // AppName -> client channels
	clientsMu sync.Mutex
)

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

	cfg, err := config.LoadConfig()
	if err != nil {
		http.Error(w, "Failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	appPrompts := make(map[string]string)
	for pr, name := range cfg.Prompts {
		appPrompts[name] = pr
	}

	gridHTML := ""
	if len(cfg.Apps) == 0 {
		gridHTML = `
        <div class="empty-state">
            <h3>No applications found</h3>
            <p>Generate a new website using the CLI:<br><code>promptyly create "your prompt here"</code></p>
        </div>`
	} else {
		gridHTML = `<div class="grid">`
		for name := range cfg.Apps {
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

			gridHTML += fmt.Sprintf(`
            <div class="card">
                <div>
                    <h3 class="card-title">%s</h3>
                    <p class="card-desc">%s</p>
                </div>
                <a href="/apps/%s/" class="card-btn">Open Application</a>
            </div>`, displayName, displayPrompt, name)
		}
		gridHTML += `</div>`
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Promptyly Hub</title>
    <link href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@300;400;600;700&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #0b0f19;
            --card-bg: rgba(22, 28, 45, 0.4);
            --border-color: rgba(255, 255, 255, 0.08);
            --text-primary: #f3f4f6;
            --text-secondary: #9ca3af;
            --accent-color: #4f46e5;
            --accent-glow: rgba(79, 70, 229, 0.3);
        }
        body {
            background-color: var(--bg-color);
            color: var(--text-primary);
            font-family: 'Plus Jakarta Sans', sans-serif;
            margin: 0;
            padding: 50px 20px;
            display: flex;
            flex-direction: column;
            align-items: center;
            min-height: 100vh;
        }
        .container {
            max-width: 1000px;
            width: 100%;
        }
        header {
            margin-bottom: 50px;
            text-align: center;
        }
        h1 {
            font-size: 3.5rem;
            font-weight: 700;
            margin: 0 0 12px 0;
            background: linear-gradient(135deg, #a5b4fc 0%, #6366f1 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            letter-spacing: -0.05em;
        }
        p.subtitle {
            color: var(--text-secondary);
            font-size: 1.15rem;
            margin: 0;
        }
        .grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
            gap: 25px;
        }
        .card {
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 20px;
            padding: 28px;
            backdrop-filter: blur(20px);
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            display: flex;
            flex-direction: column;
            justify-content: space-between;
            min-height: 180px;
        }
        .card:hover {
            transform: translateY(-6px);
            border-color: var(--accent-color);
            box-shadow: 0 15px 30px -5px var(--accent-glow);
        }
        .card-title {
            font-size: 1.35rem;
            font-weight: 700;
            margin: 0 0 12px 0;
            color: var(--text-primary);
        }
        .card-desc {
            color: var(--text-secondary);
            font-size: 0.95rem;
            line-height: 1.6;
            margin: 0 0 24px 0;
            flex-grow: 1;
        }
        .card-btn {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            padding: 12px 20px;
            background-color: var(--accent-color);
            color: white;
            text-decoration: none;
            border-radius: 10px;
            font-weight: 600;
            font-size: 0.95rem;
            transition: background-color 0.2s, transform 0.1s;
        }
        .card-btn:hover {
            background-color: #4338ca;
        }
        .empty-state {
            text-align: center;
            padding: 80px 40px;
            border: 2px dashed var(--border-color);
            border-radius: 20px;
            color: var(--text-secondary);
        }
        code {
            background: rgba(255, 255, 255, 0.05);
            padding: 6px 12px;
            border-radius: 6px;
            font-family: monospace;
            display: inline-block;
            margin-top: 15px;
            color: #a5b4fc;
            font-size: 0.95rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Promptyly Hub</h1>
            <p class="subtitle">Your locally generated, git-backed web applications</p>
        </header>
        
        %s
    </div>
</body>
</html>`, gridHTML)

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

	cfg, err := config.LoadConfig()
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

// StartDevServer launches a local static server on the specified port.
// If the port is occupied, it returns nil error (assuming it's already running in the background/another process).
func StartDevServer(defaultPort int) (int, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", defaultPort))
	if err != nil {
		// Port already bound, assuming server is running in another terminal
		return defaultPort, nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", homeHandler)
	mux.HandleFunc("/apps/", appsHandler)

	go func() {
		_ = http.Serve(ln, mux)
	}()

	return defaultPort, nil
}
