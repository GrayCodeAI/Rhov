# Memory

Memory lets Rho recall facts, decisions, and patterns from earlier sessions. Rho indexes saved information and searches it automatically, so new sessions can reuse relevant context.

---

## What Is Memory?

Without memory, each Rho session starts fresh. When memory is enabled, Rho can:

- Recall project conventions you explained before
- Reuse debugging steps that worked
- Carry architectural decisions forward across sessions
- Avoid re-asking questions it already has answers to

Memory is a local subsystem built into Rho. It stores memories as files under
Rho's state directory; no external service is required.

---

## Enabling Memory

Memory is on by default. To inspect what has been stored:

```
/learn    # lessons learned across sessions
```

### Settings

```json
// ~/.rho/settings.json
{
  "memory": {
    "enabled": true
  }
}
```

---

## How Memory Is Stored

Memory is stored under Rho's state directory (`~/.rho/` by default).

| Location | Scope | Description |
|----------|-------|-------------|
| `~/.rho/memories/` | Global | Core memories and preferences |
| `~/.rho/` state files | Workspace | Project-specific memory and lessons |

Rho combines core memories, automatic captures, evolving guidelines, and
retrieval metrics to rank what is relevant to the current turn.

---

## Working with Memory

### Remember

Ask Rho to remember something:

```
/remember always open PR links after pushing
```

Rho records entries as durable statements organized by topic.

### Forget

Ask what Rho should forget:

```
/forget the snake_case convention
```

Forget is best-effort.

### Recall

Ask what Rho remembers:

```
/what do you remember about auth?
```

Rho searches across all memory sources and summarizes.

---

## Browsing Memory

Lessons learned are surfaced through:

```
/learn
```

---

## First-Turn Injection

On the first turn of each session, Rho automatically searches memory for content relevant to the current project and injects it as context.

Configure injection:

```json
{
  "memory": {
    "first_turn_injection": true
  }
}
```

---

## Where to Go Next

| Document | What You Will Learn |
|----------|-------------------|
| [Headless Mode](14-headless-mode.md) | Non-interactive usage |
| [Agent Mode](15-agent-mode.md) | Agent configuration |

---

© 2026 GrayCode AI. All rights reserved.