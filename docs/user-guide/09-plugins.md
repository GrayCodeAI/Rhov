# Plugins

Plugins bundle skills, agents, hooks, MCP servers, and LSP configurations into one installable unit. They enable team sharing and marketplace distribution of automation.

---

## What a Plugin Contains

A plugin is a directory that can hold:

- **Skills** — in `skills/` directory
- **Agents** — in `agents/` directory
- **Hooks** — in `hooks/hooks.json`
- **MCP servers** — in `.mcp.json`
- **LSP servers** — in `.lsp.json`

For example, a `team-tools` plugin might include a deploy skill, a code-review agent, and pre-commit hooks. Install them together in one step.

---

## Plugin Locations

Rho discovers plugins from these locations, in priority order:

| Location | Scope | Trust Required |
|----------|-------|----------------|
| `.rho/plugins/` | Project | Yes |
| `~/.rho/plugins/` | User | No |
| `--plugin-dir` | Process | No |

When two plugins share a name, the higher-priority location wins.

---

## CLI Management

Manage plugins from the command line:

```bash
# List installed plugins
rho plugins list

# Install a plugin
rho plugins install user/repo
rho plugins install user/repo@v1.0
rho plugins install /path/to/local/plugin

# Install with trust (required for project plugins)
rho plugins install user/repo --trust

# Uninstall a plugin
rho plugins uninstall my-plugin

# Update plugins
rho plugins update
```

---

## Marketplace

Browse and install plugins from the community marketplace:

```bash
# Search marketplace
rho plugins search go

# Install from marketplace
rho plugins install go-review --trust
```

### Adding Marketplace Sources

Configure additional marketplaces in `~/.rho/settings.json`:

```json
{
  "marketplaces": [
    {
      "name": "Team Plugins",
      "source": {
        "git": "https://github.com/my-org/plugins.git"
      }
    }
  ]
}
```

---

## Trust Model

- **User plugins** (`~/.rho/plugins/`) — Trusted automatically
- **Project plugins** (`.rho/plugins/`) — Require explicit trust before activation

Trust is required because plugins can execute code via hooks and MCP servers. To trust a plugin:

```bash
rho plugins install <source> --trust
```

---

## Plugin Directory Layout

```
my-plugin/
  skills/
    SKILL.md
  agents/
    reviewer.yaml
    deployer.yaml
  hooks/
    hooks.json
  plugin.json        # Optional manifest
```

### plugin.json Manifest

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "Team development tools",
  "components": {
    "skills": true,
    "agents": true,
    "hooks": true
  }
}
```

---

## Where to Go Next

| Document | What You Will Learn |
|----------|-------------------|
| [Hooks](10-hooks.md) | Event-driven automation |
| [Custom Models](11-custom-models.md) | Provider configuration |

---

© 2026 GrayCode AI. All rights reserved.