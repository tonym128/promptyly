# Promptyly AI Agent Harness (`agent/`)

This document details the architecture, configuration, prompt design, and response parsing engine behind the Promptyly AI Agent harness.

---

## 1. Core Architecture

The agent harness is abstracted under a unified interface inside [agent.go](file:///home/tonym/Projects/promptyly/agent/agent.go). This abstraction allows the main application loop to operate independently of the underlying LLM provider.

```go
type Response struct {
	Files   map[string]string // Filename -> File Contents
	Summary string            // Short summary of changes made
}

type Agent interface {
	Generate(systemPrompt, userPrompt string) (*Response, error)
}
```

Clients are instantiated via a unified factory in [factory.go](file:///home/tonym/Projects/promptyly/agent/factory.go) using the configuration loaded from `~/.config/promptyly/config.json`:

```go
func NewClient(provider string, cfg config.ProviderConfig) (Agent, error)
```

---

## 2. Response Parsing Engine (`agent.go`)

To robustly extract structured directories of code files from text-based LLM outputs, Promptyly enforces an XML-like tag layout:

* **File Blocks**: `<file name="path/to/file">...</file>`
* **Change Summary**: `<summary>...</summary>`

The parser uses regular expressions to extract these blocks without needing XML DOM parses, ensuring it is resilient to trailing text or other conversational filler.

```go
var (
	fileRegex    = regexp.MustCompile(`(?s)<file\s+name="([^"]+)">([\s\S]*?)<\/file>`)
	summaryRegex = regexp.MustCompile(`(?s)<summary>([\s\S]*?)<\/summary>`)
)
```

---

## 3. System Prompts Design

The harness leverages two core system prompts depending on the operation:

### A. Application Creation Prompt
Used when generating a brand-new application from scratch. Instructs the LLM to write complete, premium web applications with visual excellence (gradients, fonts, transitions) and local persistence integrations.

* **Database Instruction**:
  Instructs the LLM that state persistence (e.g. lists, settings, user data) can be performed by sending standard relative fetch calls to `_promptyly/api/db` (`GET` to read, `POST` to store JSON data).

### B. Application Modification / Editing Prompt
Used during interactive CLI sessions when the user requests an edit (e.g. *"add a search bar"*).
* **Context Loading**: Reads all existing `.html`, `.css`, `.js`, and `.json` files in the project workspace and packages them as context for the LLM:
  ```
  --- FILE: index.html ---
  [content]
  --- FILE: styles.css ---
  [content]
  ```
* **Output Isolation**: Instructs the LLM to **only** return `<file>` blocks for files that were modified or newly created, keeping transmission tokens small and speeds fast.

---

## 4. Providers Harness Details

### 🟢 Gemini Client ([gemini.go](file:///home/tonym/Projects/promptyly/agent/gemini.go))
* **Endpoint**: Generative Language v1beta REST API.
* **Payload**: Utilizes `systemInstruction` parts to supply system guidance and the `contents` block for the user's prompt.
* **Default Model**: `gemini-1.5-flash`

### 🔵 Claude Client ([claude.go](file:///home/tonym/Projects/promptyly/agent/claude.go))
* **Endpoint**: Messages API (`/v1/messages`).
* **Headers**: Attaches `x-api-key` and `anthropic-version`.
* **Default Model**: `claude-3-5-sonnet-20240620`

### 🟣 OpenAI Compatible Client ([openai.go](file:///home/tonym/Projects/promptyly/agent/openai.go))
Handles local LLM servers (**Ollama**, **LM Studio**) and custom endpoints.
* **Auto URL-Cleaning**: Cleans inputs to strip out command prefixes, quotes, and backslashes (e.g., if a user copies a curl request from LM Studio's interface like `curl http://localhost:1234/v1/chat \`).
* **Path Resolution**: Automatically detects and handles URL routing suffix variations:
  * If the URL ends in `/chat` $\rightarrow$ resolves to `/chat/completions` (e.g., LM Studio/Llama API formats).
  * If the URL ends in `/v1` $\rightarrow$ resolves to `/v1/chat/completions`.
  * If the URL is a naked domain $\rightarrow$ resolves to `/v1/chat/completions` (Ollama's OpenAI compatibility path).
