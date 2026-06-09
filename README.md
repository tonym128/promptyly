# Promptyly CLI

**Promptyly** is a cross-platform (Mac, Linux, Windows) Go-based developer command-line interface and service that owns the `prompt://` custom URL protocol. It allows you to generate complete, beautiful, and functional single-page web applications from natural language prompts, run them locally, edit them iteratively in real-time with an LLM agent, and easily package and share them as `.zip` archives.

---

## Key Features

1. **Custom `prompt://` URL Protocol**: Owns and registers `prompt://` deep links on Linux, Windows, and macOS, letting you launch generation and dev sessions directly from your browser or other scripts.
2. **Git-Backed App Generation**: Saves generated projects locally in `~/promptyly-apps/<prompt-slug>`. It initializes a local Git repository, committing changes after each generation/edit block.
3. **Action Memory & History**: Logs a detailed audit trail of all AI actions, prompts, and modified files in `.promptyly/history.json`.
4. **Single Local Server & Hot Reloading**: All applications are hosted on a single unified port (default `6071`) at `http://localhost:6071/apps/<app-name>/`. Multiple active sessions run concurrently on the same port. HTML files are injected with a zero-dependency Server-Sent Events (SSE) reload listener that updates the browser instantly upon terminal edits.
5. **Promptyly Hub Homepage**: Visiting `http://localhost:6071/` serves a stunning dark-mode dashboard showing your registry of generated apps, their original creation prompts, and one-click launch buttons.
6. **No-Config Persistence API**: Provides a local JSON database endpoint at `http://localhost:6071/apps/<app-name>/_promptyly/api/db`. Apps can make standard `fetch` requests (`GET`/`POST`) to easily save and load state.
7. **Provider Harness**: Integrates with Gemini, Claude, Ollama, LM Studio, or any OpenAI-compatible API endpoint out of the box.
8. **Portable Zip Sharing**: Export your projects with `promptyly export` and import them on any machine running Promptyly using `promptyly import`.

---

## Installation

### 1. Compile the Binary
Ensure you have Go (1.20+) installed. Run the build command:
```bash
go build -o promptyly main.go
```

### 2. Move to Path (Optional)
Move the compiled binary to a directory in your system `$PATH` (e.g. `/usr/local/bin` or `~/.local/bin`):
```bash
mv promptyly ~/.local/bin/
```

---

## Commands Reference

### 1. AI Configuration
Run the interactive walkthrough to select your default AI provider, model, and save API keys:
```bash
promptyly config setup
```

You can also view your configuration or set keys manually:
```bash
# View config path and keys
promptyly config show

# Manually set a key
promptyly config set gemini_key "YOUR_API_KEY"
promptyly config set default_provider "gemini"
```

Available config keys: `default_provider`, `gemini_key`, `gemini_model`, `claude_key`, `claude_model`, `ollama_url`, `ollama_model`, `lmstudio_url`, `lmstudio_model`, `apps_dir`.

### 2. Build a Website
Create a website instantly from a prompt. Promptyly will contact the LLM, create files in `~/promptyly-apps/`, initialize git, start the server, open your default browser, and open an interactive editing terminal:
```bash
promptyly create "A sleek dark mode pomodoro timer with custom audio loops and task list"
```

### 3. Edit & Work on an App
Resume working on an existing application in your browser:
```bash
promptyly run a-sleek-dark-mode-pomodoro-timer
```
Once loaded, you can type updates in your terminal like `add a dark mode toggle` or `change default time to 45 mins`, and watch the browser update in real-time.

### 4. Register the Custom URL Scheme
Register the `prompt://` protocol handler in your operating system registry (Windows) or desktop database (Linux):
```bash
promptyly register
```

### 5. Open Deep Links (URL Handler)
The CLI processes `prompt://` links. This command is executed automatically by the OS when deep links are clicked:
```bash
# Triggers creation of a calculator web app
promptyly handle "prompt://create?prompt=Simple+Calculator"

# Triggers running of an existing calculator app
promptyly handle "prompt://run?name=simple-calculator"
```

### 6. Sharing & Packaging
To share an application with a friend:
```bash
promptyly export a-sleek-dark-mode-pomodoro-timer pomodoro.zip
```
Your friend can then import it on their machine:
```bash
promptyly import pomodoro.zip
```

---

## App Persistence API (Built-in JSON DB)

Generated applications can easily persist data without writing any backend code. When serving files, Promptyly hosts a JSON document store at `/_promptyly/api/db` which maps to `.promptyly/db.json` inside the app folder:

* **Retrieve Data** (`GET`):
  ```javascript
  const response = await fetch('_promptyly/api/db');
  const data = await response.json();
  ```
* **Store Data** (`POST`):
  ```javascript
  await fetch('_promptyly/api/db', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ todos: [...] })
  });
  ```
All database files are kept inside the project folder so they are automatically tracked by git and included when you zip/export your app!
