# Getting Started

Rho is an AI-powered coding agent for your terminal, built for developers by GrayCode AI. It understands your codebase, executes shell commands, edits files, searches the web, and manages tasks — all through natural language.

You can use Rho interactively as a full-screen TUI, run it headlessly for scripting and CI/CD, or integrate it into editors via the Agent Client Protocol (ACP).

---

## Installation

Rho is currently in active development. Contributor source builds are the primary path while we harden the product in the open.

### From Source (Recommended for Contributors)

```bash
git clone https://github.com/GrayCodeAI/rho && cd rho
GOWORK=off go build -o rho ./cmd/rho
./rho
```

For full cross-repository development, place the nine Go repositories beside
Rho in a parent workspace and run `make setup` from `rho`; `graycode-eco` is
only a local folder name, not a repository containing those modules.

### Verification

Run the developer path check to verify your setup:

```bash
./rho path
```

This checks setup and security readiness.

---

## First Launch

Start Rho by running:

```bash
rho
```

On first launch, Rho opens a TUI where you can configure credentials. Press `/config` (or `/autonomy`) to open the configuration picker. Your API keys are stored in your OS keychain (macOS Keychain or Linux keyring), never in plain text.

Rho supports multiple providers:

- **xAI Grok** — `XAI_API_KEY`
- **Anthropic Claude** — `ANTHROPIC_API_KEY`
- **OpenAI** — `OPENAI_API_KEY`
- **Google Gemini** — `GEMINI_API_KEY`
- **OpenRouter** — `OPENROUTER_API_KEY`

See [Authentication](02-authentication.md) for the full set of auth options including OIDC and device code flow.

---

## Basic Interaction

Once authenticated, Rho presents a full-screen TUI powered by Bubble Tea with two main areas:

- **Scrollback** — the conversation history showing your prompts, Rho's responses, tool calls, file edits, and more
- **Prompt** — the input area at the bottom where you type messages

Type a message and press `Enter` to send it. Rho reads files, runs commands, and edits code as needed. Each tool run streams into the scrollback in real time.

Press `Tab` to move focus between the prompt and the scrollback. While a turn is running, `Ctrl+C` cancels it. In Vim mode, use `j`/`k` to navigate and `h`/`l` to collapse/expand entries.

### File References

Use `@` in your prompt to attach files:

```
@src/main.go              # Attach a file
@src/main.go:10-50        # Attach lines 10-50
@src/                     # Browse a directory
```

The `@` operator opens a fuzzy file picker. By default it respects `.gitignore` and hides dotfiles.

### Permissions

Rho exposes two independent control surfaces:

- **`/autonomy`** — Controls the trust tier (Always Ask, Scout, Builder, Operator, Autonomous)
- **`/spec`** — A workflow gate that blocks Write/Edit/Bash until you approve implementation

Example:

```
/autonomy tier builder
/autonomy allow Bash(git:*)
/autonomy deny Bash(rm -rf *)
/autonomy save project
```

---

## Key Concepts

### Sessions

Every conversation is a **session**. Sessions are automatically saved and can be resumed later. Each session tracks the full conversation history, tool calls, file edits, and task state.

- Start a new session: `Ctrl+N` or `/new`
- Resume a previous session: `/resume` in the TUI, or `--resume <ID>` from the CLI
- Continue the most recent session: `rho -c`

### Scrollback

The scrollback shows:

- **User prompts** — your messages, rendered as sticky headers
- **Agent messages** — Rho's responses with markdown rendering
- **Thinking blocks** — Rho's reasoning process (collapsible)
- **Tool calls** — file edits, command executions, search results
- **Task lists** — TODO items tracking progress

### Tools

Rho has built-in tools for:

| Tool | Description |
|------|-------------|
| `Read` / `Write` / `Edit` | Read and edit files |
| `Grep` | Regex search across your codebase |
| `LS` / `Glob` | List and find files |
| `Bash` | Execute shell commands |
| `WebFetch` / `WebSearch` | Search the web and fetch URLs |
| `TodoWrite` | Create and manage task lists |
| `Agent` | Spawn parallel subagent sessions |

### Slash Commands

Type `/` in the prompt to access commands. These provide quick actions without writing a full prompt:

```
/autonomy              # Open autonomy picker
/spec                  # Start spec workflow
/compact               # Compress conversation history
/model grok-build      # Switch model
/new                   # Start a new session
/resume                # Resume a session
```

See [Slash Commands](04-slash-commands.md) for the complete reference.

---

## Common Launch Options

```bash
# Start the interactive TUI
rho

# Submit an initial prompt as the first turn
rho "fix the failing auth test and run it"

# Start in a specific project directory
rho --cwd ~/projects/my-app

# Resume a previous session
rho -r abc123

# Continue the most recent session
rho -c

# Use a specific provider/model
rho --provider openai --model gpt-4o

# Isolated worktree for changes
rho --worktree "refactor module X"

# Non-interactive (headless) mode
rho -p "Explain this codebase"

# Full auto mode
rho exec --auto full "add error handling"

# Dry-run mode (denies all tools)
rho exec --autonomy dry-run "What would this do?"
```

---

## Headless Mode

Run Rho non-interactively for scripting, CI/CD, and automation:

```bash
rho -p "Your prompt here"
```

Output formats:

| Format | Flag | Description |
|--------|------|-------------|
| `plain` | (default) | Human-readable text |
| `json` | `--output-format json` | Single JSON object with response |
| `stream-json` | `--output-format stream-json` | NDJSON event stream |

---

## Project Rules (AGENTS.md)

Add per-project instructions by creating an `AGENTS.md` file in your repository. Rho reads these files and injects their contents as a project-instructions message at the start of the conversation:

```
~/.rho/AGENTS.md           # Global rules (apply to all projects)
<repo-root>/AGENTS.md       # Repository-level rules
<cwd>/AGENTS.md             # Directory-level rules (highest priority)
```

Deeper files take precedence. Rho also reads `CLAUDE.md` files for compatibility.

---

## Where to Go Next

| Document | What You Will Learn |
|----------|-------------------|
| [Authentication](02-authentication.md) | Browser login, API keys, OIDC, external auth |
| [Keyboard Shortcuts](03-keyboard-shortcuts.md) | Complete reference for all key bindings |
| [Slash Commands](04-slash-commands.md) | All available `/` commands |
| [Configuration](05-configuration.md) | Settings and environment variables |

---

© 2026 GrayCode AI. All rights reserved.
