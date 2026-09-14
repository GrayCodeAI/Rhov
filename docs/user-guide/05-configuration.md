# Configuration

Rho reads configuration from settings files, environment variables, and has defaults for all options. This document covers the common configuration options.

---

## Precedence

Configuration is resolved in this order (highest priority first):

1. **CLI `--settings` JSON override** (`LoadSettingsWithOverride`)
2. **Per-command CLI flags** (e.g., `--provider`, `--model`)
3. **Environment variables** (only where explicitly read; there is no global env layer)
4. **Project settings** (`.rho/settings.json`, repository-safe subset only —
   `model`, `provider`, permissions, MCP servers and providers are stripped)
5. **User settings** (`~/.rho/settings.json`)
6. **Built-in defaults**

---

## Settings File

Location: `~/.rho/settings.json`

This is the main configuration file. Rho writes to it when you save changes via `/config` or `/autonomy save`.

### Basic Settings

```json
{
  "default_provider": "openai",
  "default_model": "gpt-4o",
  "deployment_routing": true
}
```

### Autonomy Configuration

```json
{
  "autonomy": {
    "tier": "builder",
    "dry_run": false
  }
}
```

**Tiers:**
- `always_ask` — prompts for permission on every tool call
- `scout` — classifier approves safe tools
- `builder` — broader tool access for development
- `operator` — full tool access for trusted operations
- `autonomous` — no permission prompts

- `devbox` — container-oriented development policy

### Agent Configuration

```json
{
  "autonomy": {
    "rules": {
      "allow": ["Bash(git:*)"],
      "deny": ["Bash(rm -rf *)"]
    }
  }
}
```

### Custom Models

Add custom model endpoints:

```json
{
  "models": {
    "my-custom-model": {
      "model": "custom-model-id",
      "base_url": "https://api.example.com/v1",
      "api_key": "sk-...",
      "name": "Display Name",
      "context_window": 128000
    }
  }
}
```

---

## Environment Variables

Key environment variables for configuration.

### Authentication

| Variable | Description |
|----------|-------------|
| `XAI_API_KEY` | xAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `GEMINI_API_KEY` | Google Gemini API key |
| `OPENROUTER_API_KEY` | OpenRouter API key |
| `OLLAMA_BASE_URL` | Ollama endpoint (no key required) |

### Features

| Variable | Description |
|----------|-------------|
| `RHO_Y0_FOLDER_TRUST` | Folder trust feature flag (default: `1`) |
| `RHO_Y0_MARKETPLACE` | Marketplace feature flag (default: `1`, set `0` to disable remote installs) |
| `RHO_DEPLOYMENT_ROUTING` | Not an environment variable: set `deployment_routing` in `settings.json` |

### Paths

| Variable | Description |
|----------|-------------|
| `RHO_HOME` | Harness home override used by identity only (default: `~/.rho`); most config paths honor `RHO_CONFIG_DIR` / `RHO_STATE_DIR` / `RHO_CACHE_DIR` instead |

---

## Project Configuration

Place configuration in `.rho/` within your repository:

| File | Purpose |
|------|---------|
| `.rho/settings.json` | Project settings (autonomy, rules) |
| `.rho/lsp.json` | LSP server configuration |
| `AGENTS.md` | Project instructions |

---

## MCP Servers

Configure MCP servers in global `~/.rho/settings.json` only — project
`.rho/settings.json` cannot register MCP servers (stripped by
`projectSafeSettings`; project automation additionally requires folder trust,
see below):

```json
{
  "mcp_servers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": { "GITHUB_PERSONAL_ACCESS_TOKEN": "ghp_xxx" }
    },
    "postgres": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-postgres", "postgresql://..."]
    }
  }
}
```

---

## Folder Trust

Folder trust controls whether project automation (hooks, plugins, MCP, LSP) can run.

### Trust Commands

```
/trust                  # Manage folder trust
/trust status           # Check trust status
/trust revoke <path>    # Revoke trust for a folder
```

### Trust Store

Location: `~/.rho/trusted_folders.toml`

```toml
[[folders]]
path = "/Users/me/projects/my-app"
trusted_at = "2026-07-16T00:00:00Z"
```

Projects must be trusted before their hooks, plugins, MCP servers, or LSP servers can execute.

---

## Where to Go Next

| Document | What You Will Learn |
|----------|-------------------|
| [Theming](06-theming.md) | TUI appearance and themes |
| [MCP Servers](07-mcp-servers.md) | External tool integrations |
| [Skills](08-skills.md) | Installing and using skills |
| [Plugins](09-plugins.md) | Multi-component plugins and marketplace |

---

© 2026 GrayCode AI. All rights reserved.
