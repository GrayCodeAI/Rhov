# Rho Control Plane (proposed → implemented core)

Status: **core wired** (spawn · lazy tools · plan/act/review).

## Goal

World-class CLI experience: **one face (Rho)**, deep engines, progressive power.

```text
Faces (TUI / headless / ACP / daemon)
        │
        ▼
┌───────────────────────────────────────┐
│  Control plane                        │
│  WorkMode  SpawnController            │
│  Lazy model surface                   │
└───────────────────┬───────────────────┘
                    ▼
              Session kernel
           (agentLoop · tools)
                    │
     eyrie · embedded token engine
```

## Pieces

### 1. SpawnController (`engine.SpawnController`)

Single entry for subagents + background tasks:

- `Spawn(ctx, SpawnRequest)` — sync
- `SpawnBackground(ctx, id, req)` — async via `taskruntime`
- `Tasks()` — unified registry
- Agent tool continues through `WireAgentTool` → same `spawnSubAgentRequest`

### 2. Lazy model surface (`tool.Registry`)

- Essential tools registered and **model-visible**
- Optional tools registered for **execution + ToolSearch**, hidden from `EyrieTools`
- `ToolSearch` `select:Name` **promotes** tool onto the model surface
- APIs: `EnableLazyModelSurface`, `SetModelVisibility`, `PromoteModelTool`

### 3. WorkMode plan / act / review

| Mode | Tools | Bash |
|------|-------|------|
| `act` | Essential model set | full |
| `plan` | Plan set (read + plan suite) | read-only allowlist |
| `review` | Review set | read-only allowlist |

- API: `Session.SetWorkMode`, `Session.WorkMode()`
- TUI: `/mode plan|act|review` (shell modes remain `auto|shell|agent`)
- Ephemeral system prompt addon injected in `agentLoop`

## User commands

```text
/mode                 # show work + shell mode
/mode plan|act|review
/mode auto|shell|agent
```

## Iteration 2 (additional surface)

### Folder trust UX
- Already enforced in hooks/plugins (PACK-03); now surfaced in product:
  - `engine.ProjectTrust` / `TrustProject` / `UntrustProject`
  - TUI `/trust [status|add|remove]`
  - Status bar shows `trusted` / `untrusted` / `trust:off`

### Onboarding
- `/start` — trust, work mode, git advice, first tasks
- `/start trust` — trust cwd
- `/start branch` — create agent branch from main

### Git safety
- `engine.InspectGitBranch` / `EnsureAgentBranch`
- `/branch-agent` when on main/master
- Status bar warns `main!` when on main

### HUD
- Wide status secondary row: `mode:act · trusted`
- `/status` includes work mode, trust, visible tool counts, git advice

## Iteration 3

### Auto-commit productized
- `ToolService.SetAutoCommit` → `ToolContext.AutoCommit` (was never wired from CLI)
- `--auto-commit` flag + `settings.auto_commit` + `/auto-commit on|off|status`
- Status bar shows `auto-commit` when enabled
- Write/Edit/StructuredEdit already call `tool.AutoCommit` when flag is set

### Background tasks
- Production path: `BackgroundAgentManager` → `taskruntime` (SpawnController uses same)
- `BackgroundAgentPool` remains test/legacy reexport only (not chat session path)

## Iteration 4

### Onboarding / CI
- CLI `Welcome` surfaces control-plane commands + `rho exec`
- Example workflow: `examples/github/rho-ci-exec.yml`

### ACP
- `initialize` advertises `rhoCapabilities` (work modes, lazy tools, …)
- `session/new` returns `rho` snapshot (`workMode`, `autoCommit`) and defaults act mode
- `session/setMode` — switch work mode (plan|act|review)
- `session/status` — control-plane snapshot (mode, autoCommit, message count)

### Deprecations
- `BackgroundAgentPool` / `NewBackgroundAgentPool*` / `FormatResults` marked Deprecated
  in favor of `Session.SpawnController()` (same taskruntime.Registry). Retained
  for compatibility; no production callers found.

## Not done yet (next iterations)

- True 60s binary install path (packaging/CI)
- Deeper ACP (session/setMode, client fs routing)

- Public Terminal-Bench scorecard
- Optional: deprecate BackgroundAgentPool reexports

## Tests

- `internal/engine/control_plane_test.go`
- `internal/engine/project_trust_test.go` / git safety tests
- Permission display + diff tests
