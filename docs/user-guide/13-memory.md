# Memory

Memory lets Hawk recall facts, decisions, and patterns from earlier sessions. Hawk indexes saved information and searches it automatically, so new sessions can reuse relevant context.

---

## What Is Memory?

Without memory, each Hawk session starts fresh. When memory is enabled, Hawk can:

- Recall project conventions you explained before
- Reuse debugging steps that worked
- Carry architectural decisions forward across sessions
- Avoid re-asking questions it already has answers to

Memory is a local subsystem built into Hawk. It stores memories as files under
Hawk's state directory; no external service is required.

---

## Enabling Memory

Memory is on by default. To inspect what has been stored:

```
/learn    # lessons learned across sessions
```

### Settings

```json
// ~/.hawk/settings.json
{
  "memory": {
    "enabled": true
  }
}
```

---

## How Memory Is Stored

Memory is stored under Hawk's state directory (`~/.hawk/` by default).

| Location | Scope | Description |
|----------|-------|-------------|
| `~/.hawk/memories/` | Global | Core memories and preferences |
| `~/.hawk/` state files | Workspace | Project-specific memory and lessons |

Hawk combines core memories, automatic captures, evolving guidelines, and
retrieval metrics to rank what is relevant to the current turn.

---

## Working with Memory

### Remember

Ask Hawk to remember something:

```
/remember always open PR links after pushing
```

Hawk records entries as durable statements organized by topic.

### Forget

Ask what Hawk should forget:

```
/forget the snake_case convention
```

Forget is best-effort.

### Recall

Ask what Hawk remembers:

```
/what do you remember about auth?
```

Hawk searches across all memory sources and summarizes.

---

## Browsing Memory

Lessons learned are surfaced through:

```
/learn
```

---

## First-Turn Injection

On the first turn of each session, Hawk automatically searches memory for content relevant to the current project and injects it as context.

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