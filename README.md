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

You can install the pre-compiled Promptyly CLI binary directly via our automated install script. The script dynamically detects your operating system and CPU architecture (supporting AMD64, ARM/ARM64, and Android Termux), downloads the latest binary from the Sharing Registry, configures the registry server URL, and automatically registers the custom `prompt://` URL scheme handler. 

During the installation process, the installer will also walk you through setting up your LLM configuration. You can choose to configure a remote LLM API/URL (such as Gemini, Claude, Ollama, or an OpenAI-compatible endpoint) or select the option to automatically download and configure a lightweight, CPU-optimized local coding model (**Qwen2.5-Coder-1.5B** in **llamafile** format, which runs smoothly on machines with as little as 4GB RAM).

### One-Line Installers (Recommended)

* **macOS / Linux / Android (Termux)**:
  ```bash
  curl -fsSL http://localhost:6072/install.sh | sh
  ```
* **Windows (PowerShell)**:
  ```powershell
  irm http://localhost:6072/install.ps1 | iex
  ```
*(Note: Replace `http://localhost:6072` with the actual public URL of your registry server when installing in production)*

---

### Local Development Build
If you prefer to compile the binaries locally and set up the development environment (including browser extension packaging):

#### 1. Run the Unified Builder
* **Linux / macOS**:
  ```bash
  ./build_all.sh
  ```
* **Windows (PowerShell)**:
  ```powershell
  .\build_all.ps1
  ```
*(These scripts compile the local Go binaries, restore node dependencies for the desktop environment, and package the browser extension inside the `dist/` directory)*

#### 2. Move Daemon to System PATH (Optional)
To run the CLI tool globally from any directory:
* **Linux / macOS / Android**:
  ```bash
  mv promptyly ~/.local/bin/  # Or another path in your $PATH (already in PATH if using Termux)
  ```
* **Windows**: Add the folder containing `promptyly.exe` to your user **Environment Variables -> PATH**.

After local building, you can manually register the `prompt://` custom URL scheme:
```bash
promptyly register
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
To resume working on an existing application in your browser and terminal:
```bash
promptyly run a-sleek-dark-mode-pomodoro-timer
```
Once loaded, you can type updates in your terminal like `add a dark mode toggle` or `change default time to 45 mins`, and watch the browser update in real-time.

#### How App Editing Works Under the Hood
1. **Local Server Hosting**: The app is served from the `~/promptyly-apps/<app-name>` directory at `http://localhost:6071/apps/<app-name>/`.
2. **Git Version Control**: Each edit you make is automatically committed to a local Git repository initialized in your app's directory. This allows you to track changes, view history, or revert if needed.
3. **Hot Reload Injection**: The server automatically injects a lightweight Server-Sent Events (SSE) listener into your app's HTML.
4. **Interactive Promptyly Console**: When editing or running an app, you enter an interactive console (`promptyly>`).

#### Shortcut Commands
While busy prompting the app in the interactive `promptyly>` terminal, you can issue the following shortcut commands directly to take actions:
* **`.publish [optional description]`**: Publishes/uploads the application to the configured remote sharing registry server.
  * If a description is not supplied inline, you will be prompted for it.
* **`.reload`**: Manually triggers a browser hot-reload for the application.
* **`.exit`** (or **`exit`**): Stops the local dev server and safely exits the interactive session.

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
2. **Local Daemon Server & Web UI**: A unified HTTP server (defined in [server/server.go](file:///home/tonym/Projects/promptyly/server/server.go)) hosting:
   * **Promptyly Hub**: The dark-mode dashboard home page, featuring local application registry search and configuration panels.
   * **Web Apps**: Renders generated sites dynamically.
   * **Hot Reloading & JSON DB**: Embeds SSE reload triggers and serves local persistence DB queries.

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

### Registry Server Security Options

The Sharing Registry Server supports robust security and user management configurations via environment variables:

* **Admin Account Generation**: On first startup, the server automatically generates a default administrator account (`admin`) and writes a secure, unique password to the server logs. You can override or pre-configure this via:
  - `ADMIN_USERNAME`: The custom username for the server admin (default: `admin`).
  - `ADMIN_PASSWORD`: The custom password for the server admin.
* **Disable Self-Registration**: Restrict account creation by setting:
  - `ALLOW_SELF_REGISTRATION=false`: Disables the `/register` web form and API. Registration attempts will return `403 Forbidden`.
* **Require Admin Approval**: Set to hold new registrations for review:
  - `REQUIRE_ADMIN_APPROVAL=true`: New registrations default to pending approval. They cannot log in, get an API token, or publish apps until approved by an admin.
  - **Admin Panel**: When signed in as an administrator, an "Admin Panel" link will appear in the navigation bar, allowing you to list, approve, or delete registered user accounts.
* **Require Login to View Registry**: Secure the entire registry website and API by setting:
  - `REQUIRE_LOGIN_TO_VIEW=true`: Anonymous users cannot view apps, search, or access the gallery. Web routes redirect to `/login`, and REST APIs return `401 Unauthorized`. (Core installer scripts and public binaries remain open so client machines can still install the CLI).

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

## 🛠️ Daemon & Docker Services

To make Promptyly easy to launch and run, the following utilities and environments are provided:

1. **One-Command Startup Scripts**:
   - Run **[`./start.sh`](file:///home/tonym/Projects/promptyly/start.sh)** (Mac/Linux) or **[`./start.ps1`](file:///home/tonym/Projects/promptyly/start.ps1)** (Windows) to automatically compile the Go daemon, start the background server locally, and open the Hub dashboard Web UI in your default browser.
2. **Docker Compose Stacks (Sharing Registry & Services)**:
   - **Default Stack** (runs the Sharing Registry server): Run `docker compose up -d` using **[`docker-compose.yml`](file:///home/tonym/Projects/promptyly/docker-compose.yml)**.
   - **Full Stack** (includes registry + containerized Ollama + DeepSeek Coder 1.3B): Run `docker compose -f docker-compose-full.yml up -d` using **[`docker-compose-full.yml`](file:///home/tonym/Projects/promptyly/docker-compose-full.yml)**.
   
   #### Prerequisite: Directory Permissions
   Before starting Docker, ensure the sharing data folder is writable by the container:
   ```bash
   sudo chown -R $USER:$USER ./sharing/data
   ```

   #### Local LLM Loopback Access (LM Studio & Ollama)
   Since the `promptyly` daemon runs locally as a native host binary, it can connect directly to local LLM engines running on the host (like LM Studio) or the containerized Ollama service:
   - **OpenAI-compatible / LM Studio Endpoint**: `http://127.0.0.1:1234/v1`
   - **Ollama Endpoint (DeepSeek Coder 1.3B)**: `http://127.0.0.1:11434`

3. **Packaging Utility**:
   - Run **[`./package.sh`](file:///home/tonym/Projects/promptyly/package.sh)** to cross-compile Go daemon binaries for Windows, macOS, and Linux, while bundling the browser extension as a ready-to-release ZIP archive in `dist/`.

### Llamafile Server Hosting (Local Cache)

To support offline installations or speed up setup processes, you can host the **Qwen2.5-Coder-1.5B** llamafile model directly on your Sharing Registry server. When configured, client installations and configuration commands will automatically fetch the model from your local server instead of Hugging Face.

1. **Docker Build/Run Process**:
   Configure this via the `INCLUDE_LLAMAFILE` environment variable at image build time:
   ```bash
   INCLUDE_LLAMAFILE=true docker compose build
   # Or build and run directly:
   INCLUDE_LLAMAFILE=true docker compose up -d --build
   ```

2. **Local Build/Packaging**:
   Set `INCLUDE_LLAMAFILE=true` before running the compilation and packaging scripts:
   - **Linux / macOS**:
     ```bash
     export INCLUDE_LLAMAFILE=true
     ./build_all.sh
     ```
   - **Windows (PowerShell)**:
     ```powershell
     $env:INCLUDE_LLAMAFILE="true"
     .\build_all.ps1
     ```

Once hosted, client scripts (`install.sh`, `install.ps1`) and CLI setup commands (`promptyly config setup`, `.llm download`) will auto-detect the local copy on the registry server and use it for downloads.

---

## 🔄 Auto-Updates & Version Checks

Promptyly features a fully automatic update mechanism to keep client CLIs and services up-to-date:

1. **Automatic Updates on Next Run**:
   When starting the background daemon (`promptyly serve`) or initiating an interactive edit session (`promptyly run` or `promptyly create`), the CLI queries the configured registry server for updates in the background. If a newer version is available:
   - It downloads the appropriate binary for your OS and architecture automatically in the background.
   - It replaces the running executable on disk. On Windows, a background PowerShell script handles swapping the files on exit when the file lock is released.
   - The update completes silently and takes effect on your **next run**.

2. **WebUI Update Banner**:
   Opening the Hub dashboard (`http://localhost:6071`) triggers a version check in the browser. If a new version is found, a banner alert will display at the top of the interface with an **Update Now** link pointing to the server's installer scripts.

