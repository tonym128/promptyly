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

## Installation & Setup

You can build and set up the entire Promptyly developer suite (Go Daemon, Sharing Registry, Desktop App dependencies, and Browser Extension) using a single command:

### 1. Run the Unified Builder
* **Linux / macOS**:
  ```bash
  ./build_all.sh
  ```
* **Windows (PowerShell)**:
  ```powershell
  .\build_all.ps1
  ```
*(These scripts compile the local Go binaries, restore node dependencies for the desktop environment, and package the browser extension inside the `dist/` directory)*

### 2. Move Daemon to System PATH (Optional)
To run the CLI tool globally from any directory:
* **Linux / macOS**:
  ```bash
  mv promptyly ~/.local/bin/  # Or another path in your $PATH
  ```
* **Windows**: Add the folder containing `promptyly.exe` to your user **Environment Variables -> PATH**.

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

### 4. Register/Unregister the Custom URL Scheme
Register the `prompt://` protocol handler in your operating system registry (Windows) or desktop database (Linux):
```bash
promptyly register
```
Or unregister it:
```bash
promptyly unregister
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

---

## Architecture & Services

Promptyly is structured as a multi-component workspace:

1. **Go CLI Engine (`promptyly`)**: The core application engine (defined in [main.go](file:///home/tonym/Projects/promptyly/main.go)) that manages AI generation, local Git version control, app deep links registration, and interactive edit sessions.
2. **Local Daemon Server**: A unified HTTP server (defined in [server/server.go](file:///home/tonym/Projects/promptyly/server/server.go)) hosting:
   * **Promptyly Hub**: The dark-mode dashboard home page.
   * **Web Apps**: Renders generated sites dynamically.
   * **Hot Reloading & JSON DB**: Embeds SSE reload triggers and serves local persistence DB queries.
3. **Desktop Application**: An Electron-based visual wrapper located in the [desktop/](file:///home/tonym/Projects/promptyly/desktop) directory offering a dual side-by-side app preview, chat sidebar interface, and setting panels.

---

## Desktop App Build & Run

To run the visual desktop environment locally:

1. Compile the main Go binary:
   ```bash
   go build -o promptyly main.go
   ```
2. Navigate to the desktop folder and launch the app:
   ```bash
   cd desktop
   npm install
   npm start
   ```

---

## Sharing Registry Integration

Promptyly includes a remote sharing registry server (configured by default to port `6072`). You can publish your creations, search other developers' apps, or download them to your local environment.

### CLI Commands for Sharing
* **Publish an app to the registry**:
  ```bash
  promptyly publish <app-name>
  ```
  *(If your API token is not yet configured, the CLI will interactively guide you to register, log in, or paste a token)*
* **Search the remote registry**:
  ```bash
  promptyly search "<query>"
  ```
* **Download and install a shared app locally**:
  ```bash
  promptyly download <app-id>
  ```

---

## Browser Extension Interceptor

A native browser extension (Manifest V3 compatible) is available under `/browser-extension`. It intercepts all link clicks matching the custom `prompt://` scheme and opens a premium dashboard modal containing:
* App details, description, original prompt, and creator username.
* An **Open Application** button (if the app is already installed locally).
* An **Install Locally** button (which commands the local daemon to download and initialize the app).
* A **Generate Locally** button (if the link specifies a new app request with an encoded prompt).

To load it in your browser:
* **Chrome**: Navigate to `chrome://extensions/`, enable Developer Mode, and click "Load unpacked", choosing the `browser-extension` folder.
* **Firefox**: Navigate to `about:debugging#/runtime/this-firefox`, click "Load Temporary Add-on", and select the `manifest.json` file.

---

## 🛠️ Desktop Distribution & Scripts

To make Promptyly easy to launch and distribute on desktop machines, the following utilities have been added:

1. **One-Command Startup Scripts**:
   - Run **[`./start.sh`](file:///home/tonym/Projects/promptyly/start.sh)** (Mac/Linux) or **[`./start.ps1`](file:///home/tonym/Projects/promptyly/start.ps1)** (Windows) to automatically compile the Go daemon, start the background server, and boot the Electron UI app in one go.
2. **Unified Docker Environment**:
   - Run `docker compose up -d` using the root **[`docker-compose.yml`](file:///home/tonym/Projects/promptyly/docker-compose.yml)** to launch the developer daemon, the registry server, and an integrated ngrok tunnel concurrently.
3. **Packaging Utility**:
   - Run **[`./package.sh`](file:///home/tonym/Projects/promptyly/package.sh)** to cross-compile Go daemon binaries for Windows, macOS, and Linux, and output them to the Electron `bin/` directory for desktop compilation, while bundling the browser extension as a ready-to-release ZIP archive.
