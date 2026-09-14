# Plugin Development Guide

## Overview

Rho supports three plugin types:

1. **Simple plugins** — shell scripts with a `plugin.json` manifest. Tools are executed as one-shot subprocesses via stdin/stdout.
2. **Managed subprocess plugins** — binary executables managed by `PluginManager`. Supports security scanning, timeout enforcement, and auto-discovery from `~/.rho/plugins/`.
3. **Daemon plugins** — long-lived processes communicating via JSON-RPC 2.0 over stdin/stdout. Managed by `DynamicPluginManager` with full lifecycle (Discover → Load → Activate → Failed/Disabled).

## Plugin Manifest

### V1 (simple/subprocess)

Place a `plugin.json` in your plugin directory:

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "Does X",
  "commands": [
    {"name": "do-x", "command": "./do-x.sh", "description": "Runs X"}
  ]
}
```

### V2 (advanced, with hooks + daemon mode)

For hooks, daemon mode, dependencies, and configuration:

```json
{
  "name": "my-plugin",
  "version": "2.0.0",
  "description": "Advanced plugin",
  "mode": "subprocess",
  "entrypoint": "./my-plugin",
  "hooks": [
    {"event": "file.write", "command": "./on-file-write.sh"},
    {"event": "session.end", "command": "./on-session-end.sh", "async": true}
  ],
  "config": {
    "api_key_env": "MY_PLUGIN_KEY"
  },
  "dependencies": ["git"],
  "repository": "https://github.com/user/my-plugin",
  "license": "MIT"
}
```

#### Hook priority

Hooks can specify `priority` (higher runs first). Event data is passed as environment variables (`RHO_EVENT=file.write`, `RHO_FILE_PATH=/path/to/file`).

#### Daemon mode

Set `"mode": "daemon"` with `"entrypoint": "./my-daemon"`. The daemon receives JSON-RPC 2.0 requests on stdin and writes responses to stdout.

## Skills

Skills are Markdown files with YAML frontmatter placed in `.rho/skills/<name>/SKILL.md`:

```yaml
---
name: my-skill
description: What this skill does
auto-invoke: true
match:
  paths: ["**/*.py"]
  context: ["django", "orm"]
chain:
  before: ["lint"]
  after: ["test"]
---
Skill instructions here using markdown...
```

### Skill features

- **Auto-invoke**: Skills with `auto-invoke: true` and matching path/context globs are automatically activated.
- **Chaining**: Skills can declare `chain.before` and `chain.after` for ordered execution.
- **References**: Use `@ref(path/to/doc.md)` to reference supporting documents within a skill.
- **Cross-agent compatibility**: Skills follow the Agent Skills spec (agentskills.io) and work with rho, Claude Code, and Codex.

## Event Hooks

Available events for plugin and skill hooks:

| Event | Data | Description |
|-------|------|-------------|
| `session.start` | `RHO_SESSION_ID` | Session began |
| `session.end` | `RHO_SESSION_ID`, `RHO_DURATION_MS` | Session ended |
| `turn.start` | `RHO_TURN_NUM` | Agent turn started |
| `turn.end` | `RHO_TURN_NUM`, `RHO_TOOL_COUNT` | Agent turn ended |
| `tool_call.start` | `RHO_TOOL_NAME`, `RHO_TOOL_INPUT` | Tool execution started |
| `tool_call.end` | `RHO_TOOL_NAME`, `RHO_TOOL_RESULT` | Tool execution finished |
| `tool_call.error` | `RHO_TOOL_NAME`, `RHO_ERROR` | Tool execution failed |
| `file.read` | `RHO_FILE_PATH` | File was read |
| `file.write` | `RHO_FILE_PATH` | File was written |
| `file.edit` | `RHO_FILE_PATH` | File was edited |
| `file.delete` | `RHO_FILE_PATH` | File was deleted |
| `compaction.start` | `RHO_COMPACTION_REASON` | Context compaction began |
| `compaction.end` | `RHO_COMPACTION_NEW_LENGTH` | Context compaction finished |
| `budget.warning` | `RHO_BUDGET_USAGE_PCT` | Token/cost budget warning |
| `budget.exceeded` | `RHO_BUDGET_TYPE` | Budget exceeded (hard stop) |
| `error.occurred` | `RHO_ERROR_TYPE` | Error occurred |
| `error.recovered` | `RHO_RECOVERY_STRATEGY` | Error was recovered |
| `model.switch` | `RHO_NEW_MODEL` | Active model changed |
| `provider.switch` | `RHO_NEW_PROVIDER` | Active provider changed |
| `user.input` | `RHO_USER_MESSAGE` | User sent a message |
| `agent.response` | `RHO_RESPONSE_LENGTH` | Agent generated a response |

## Security

All plugins are scanned on install for:

- **Malware patterns** (`internal/plugin/malware_check.go`): eval, exec, pipe-to-shell, base64 decode to shell, hex payloads, reverse shell, netcat reverse shell. Violations → blocked.
- **Dangerous Unicode** (`internal/plugin/audit.go`): BiDi overrides (critical), zero-width chars (warning), homoglyphs (info). Critical/warning characters are stripped.
- **Permission mismatches**: Plugin binaries must not request more permissions than the plugin manifest declares.

## Publishing to the Community Registry

1. Push your plugin to a public GitHub repository.
2. The registry index (maintained at `starling/registry.json`) is periodically refreshed.
3. Users discover plugins via `rho plugin search <query>`.
4. Users install via `rho plugin install <github-repo-url>`.
5. Registry installation includes automatic audit and security scanning.

## Local Development

```bash
# Create a plugin directory
mkdir -p ~/.rho/plugins/my-plugin
cd ~/.rho/plugins/my-plugin

# Create a manifest
cat > plugin.json << 'EOF'
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "My first plugin",
  "commands": [
    {"name": "hello", "command": "echo hello", "description": "Prints hello"}
  ]
}
EOF

# List installed plugins
rho plugin list

# Run a plugin command
rho plugin exec my-plugin hello

# Create a skill (project-local)
mkdir -p .rho/skills/my-skill
cat > .rho/skills/my-skill/SKILL.md << 'EOF'
---
name: my-skill
description: My first skill
---
Do something useful.
EOF
```

## Feedback and Learning

Users can rate skills (1–5 stars) using `/skill rate <name> <rating>`. Ratings are stored in `~/.rho/feedback.json` and used by the `/learn` command to recommend skills based on project signals.
