# Promptyly Desktop Application

This directory contains the desktop wrapper for **Promptyly**, built using **Electron**, **HTML5**, **Vanilla CSS**, and **JavaScript**. 

It runs alongside the Go backend, launching the local developer server automatically as a background daemon, registering system-level `prompt://` custom protocols, and letting you create, edit, run, and package single-page web applications from a beautiful visual workspace.

---

## Key Features

1. **Dashboard Manager**: View a visual grid of all your generated applications, showing their original prompts and meta information.
2. **Interactive Preview Frame**: Run applications directly within the desktop window using an embedded iframe. The preview automatically hot-reloads whenever modifications are applied.
3. **Iterative AI Chat Sidebar**: Type prompts (e.g., *"add a reset button"*, *"make the theme dark mode"*) directly next to the running app preview, click **Apply Edits**, and watch your app update in real-time.
4. **Options & Configuration**:
   - **AI Provider Setup**: Select providers (Gemini, Claude, Ollama, LM Studio), input API keys or URL endpoints, and choose target models in a simple settings UI.
   - **Protocol Registration**: One-click register the `prompt://` custom deep link scheme to launch the desktop app directly from links.
   - **Run in the Background**: Run the app in the system tray when closing the window so your hosted web apps remain accessible on `http://localhost:6071/` in the background.
5. **App Controls**:
   - **Link Apps**: Link any existing folder on your computer that contains an `index.html` file into the Promptyly registry.
   - **Export as Zip**: Packages your app code (excluding git history) into a portable `.zip` archive.
   - **Rename & Delete**: Rename folders or unlink/delete them entirely, with options to wipe files from disk.

---

## How to Run

### 1. Install Dependencies
Make sure you have Node.js installed. From this `desktop` directory, run:
```bash
npm install
```

### 2. Start the App
Start the Electron desktop application:
```bash
npm start
```
Upon startup, the desktop app will automatically search for the `./promptyly` Go binary in the root directory. If the background server is not already running on port `6071`, it will spawn it as a daemon.

---

## Deep Link Protocols (`prompt://`)

When you register the protocol association (via **Settings -> System & OS Options**), clicking deep links on your computer will open the desktop app directly:

* **Create deep link**:
  ```
  prompt://create?prompt=Sleek+Calculator+with+neon+colors
  ```
  This opens the "Create New App" screen, prefills the prompt input, and highlights it for generation.

* **Run deep link**:
  ```
  prompt://run?name=sleek-calculator
  ```
  This opens the "App Session" preview and interactive editing window for the calculator application.
