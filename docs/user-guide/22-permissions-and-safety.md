# Permissions and Safety Controls

Hawk can read files, edit code, and run shell commands. The permission system controls what the agent is allowed to do.

---

## Permission Pipeline

When the model requests a tool:

1. **Dry-run** — Denies every tool call
2. **PreToolUse hooks** — Can deny before other checks
3. **Spec-stage gate** — Restricts tools until implementation is approved
4. **Explicit remembered rules** — Deny/allow decisions are checked before autonomy
5. **Autonomy policy** — Determines whether an otherwise-unruled call prompts
6. **High-risk approval gate** — Optional second confirmation for network/destructive actions

---

## Autonomy Tiers

| Tier | Behavior |
|------|----------|
| `always_ask` | Prompt for every tool |
| `scout` | Classify and approve safe tools |
| `builder` | Broader tool access |
| `operator` | Full tool access (trusted) |
| `autonomous` | No normal prompts; hooks, explicit rules, spec gates, and high-risk approval still apply |

Set with:

```
/autonomy tier builder
```

---

## Rule Matching

### Bash Rules

```bash
# Prefix matching
Bash(git *) — matches `git status`, `git commit`, etc.

# Glob matching
Bash(git * main) — matches `git checkout main`
```

### Path Rules

```toml
# Edit rules
Edit(src/**/*.go) — matches Go files under src/
Read(/Users/**/secrets/*) — matches secrets anywhere
```

### MCP Rules

```toml
MCPTool(linear__*) — matches all tools from linear server
```

---

## Enforcement Layers

Permissions control what the model can request; tool-level guards enforce the
hard limits:

| Layer | Controls |
|-------|----------|
| Rules | Tool access |
| Hooks | Pre-execution blocking |
| Tool guards | Destructive-command blocking, path guard, sensitive-path protection |

Recommended combination:
- restrictive rules
- PreToolUse hooks
- explicit `--add-dir` scoping

---

## Hook-based Security

Create a `PreToolUse` hook to enforce allow lists:

```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "Bash",
      "hooks": [{"type": "command", "command": "bin/safe-shell.sh"}]
    }]
  }
}
```

See [Hooks](10-hooks.md) for hook authoring.

---

## Best Practices

1. **Use narrow patterns** — More specific rules are safer
2. **Combine layers** — Rules + hooks + tool guards
3. **Review project config** — Unknown repos may have allow rules
4. **Test policies** — Verify with `dontAsk` mode
5. **Trust model** — Require trust for project automation

---

## Where to Go Next

| Document | What You Will Learn |
|----------|-------------------|
| [Dashboard](23-dashboard.md) | HUD and monitoring |
| [Monitoring Usage](24-monitoring-usage.md) | Telemetry |

---

© 2026 GrayCode AI. All rights reserved.
