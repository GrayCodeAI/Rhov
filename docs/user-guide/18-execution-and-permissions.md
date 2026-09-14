# Execution & Permissions

Rho executes agent commands directly on the host. There is no container or
OS-level sandbox layer; the safety boundary is the permission engine plus the
tool-level guards described below.

---

## Quick Start

```bash
# Set the autonomy tier (how much Rho may do without asking)
/autonomy tier builder

# Allow or deny specific tool rules
/autonomy allow Bash(git:*)
/autonomy deny Bash(rm -rf *)

# Deny every tool call unconditionally (preview only)
/autonomy dry-run on
```

---

## Autonomy Tiers

| Tier | Behavior |
|------|----------|
| `Always Ask` | Prompts for permission on every tool call |
| `Scout` | Reads auto-approve; edits and commands ask first |
| `Builder` | Reads and file changes auto-approve; commands ask first |
| `Operator` | Reads, edits, and normal commands auto-run; risky actions ask first |
| `Autonomous` | Minimal prompts; only highest-risk actions stop |

Bare `/autonomy` opens a picker. See [Permissions and Safety](22-permissions-and-safety.md)
for the full model.

---

## Tool-Level Guards

Independent of the autonomy tier, Rho always enforces:

- **Destructive-command blocking** — patterns like `rm -rf /` are hard-blocked
  in Bash and PowerShell.
- **Path guard** — file tools are restricted to the working directory and any
  directories added with `--add-dir` / `/add-dir`.
- **Sensitive-path protection** — credential files (`~/.ssh/*`, provider state,
  Rho env files) are blocked for Read/Edit/Write.
- **Environment scrubbing** — provider API keys are removed from child-process
  environments.

---

## Dry-Run Kill Switch

`/autonomy dry-run on` denies every tool call unconditionally, regardless of
tier or spec stage. Useful for CI/headless previews.

---

## Folder Trust

`/trust` manages folder trust for project automation. Hooks and project-local
plugins only load in trusted folders.

---

## Where to Go Next

| Document | What You Will Learn |
|----------|-------------------|
| [Permissions and Safety](22-permissions-and-safety.md) | The full permission model |
| [Sessions](17-sessions.md) | Session persistence |

---

© 2026 GrayCode AI. All rights reserved.