# MCP Servers

MCP (Model Context Protocol) servers extend Rho with external tool integrations. They let Rho interact with any service that implements the MCP standard.

---

## What Are MCP Servers?

An MCP server is a process that exposes tools to Rho over a standardized protocol. When you configure an MCP server, its tools become available to the model alongside Rho's built-in tools. The model can discover and call these tools during a session.

For example, a GitHub MCP server might expose tools like `create_issue`, `list_pull_requests`, and `search_code`. A database server might expose `query`, `list_tables`, and `describe_schema`.

See the [MCP specification](https://modelcontextprotocol.io) for protocol details.

---

## Configuration

MCP servers are configured in `.rho/settings.json` under the `mcp_servers` key.

### stdio Transport (Local Process)

Rho spawns a local process and communicates over stdin/stdout:

```json
{
  "mcp_servers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/dir"],
      "enabled": true
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "ghp_..."
      }
    }
  }
}
```

### HTTP/SSE Transport (Remote Server)

For remote MCP servers accessible over HTTP:

```json
{
  "mcp_servers": {
    "linear": {
      "url": "https://mcp.linear.app/mcp",
      "transport": "http"
    }
  }
}
```

---

## CLI Management

Manage MCP servers from the command line:

```bash
# List configured MCP servers
rho mcp list

# Add a stdio server
rho mcp add filesystem -- npx -y @modelcontextprotocol/server-filesystem /path/to/dir

# Add a remote HTTP server
rho mcp add linear --transport http https://mcp.linear.app/mcp

# Remove a server
rho mcp remove github

# Diagnose server configuration
rho mcp doctor
rho mcp doctor <server-name>
```

Use `--scope project` to write to `.rho/settings.json` in the current directory instead of the user config.

---

## Project-Scoped MCP Servers

MCP servers can be configured per-project in `.rho/settings.json`:

```
my-project/
  .rho/
    settings.json
  src/
  ...
```

```json
// .rho/settings.json
{
  "mcp_servers": {
    "linear": {
      "url": "https://mcp.linear.app/mcp",
      "transport": "http"
    }
  }
}
```

When a server with the same name is defined globally and in the project, the project version takes precedence.

**Trust Requirement:** Project-scoped MCP servers only load when the folder is trusted. See [Folder Trust](05-configuration.md#folder-trust) for details.

---

## Tool Naming

MCP tools are namespaced with the server name to avoid collisions:

- Server `filesystem` with tool `read_file` → `filesystem__read_file`
- Server `github` with tool `create_issue` → `github__create_issue`

---

## Discovering and Using Tools

Rho provides built-in tools to work with MCP servers:

- **`search_tool`** — Discover available integration tools across all enabled MCP servers
- **`use_tool`** — Call an integration tool discovered via `search_tool`

Example usage:

```
/search_tool github
/use_tool github__create_issue {"title": "Bug fix", "description": "..."}
```

---

## Available MCP Servers

A partial list of MCP servers you can configure:

| Server | Transport | Endpoint / Package |
|--------|-----------|--------------------|
| Linear | HTTP (OAuth) | `https://mcp.linear.app/mcp` |
| Sentry | HTTP (OAuth) | `https://mcp.sentry.dev/mcp` |
| Filesystem | stdio | `@modelcontextprotocol/server-filesystem` |
| Git | stdio | `@modelcontextprotocol/server-git` |
| GitHub | stdio | `@modelcontextprotocol/server-github` |
| PostgreSQL | stdio | `@modelcontextprotocol/server-postgres` |
| SQLite | stdio | `@modelcontextprotocol/server-sqlite` |

See the [MCP Server Registry](https://github.com/modelcontextprotocol/servers) for the full list.

---

## Where to Go Next

| Document | What You Will Learn |
|----------|-------------------|
| [Skills](08-skills.md) | Installing and using skills |
| [Plugins](09-plugins.md) | Multi-component plugins and marketplace |

---

© 2026 GrayCode AI. All rights reserved.