# Plan: bring Tau's feature set into Rho (Go)

Status: in progress
Branch: `feat/rename-rho`

## Principle

Tau's features must exist in Rho **without leaving Go**. Rho already has a
superset of Tau's agent capabilities; this plan closes the specific gaps where
Tau offers a documented feature Rho does not.

## Gap analysis (Tau feature → Rho status)

| Tau feature | Rho status | Action |
|---|---|---|
| Interactive TUI | ✅ Bubble Tea TUI | none |
| `-p` print mode | ✅ `-p` | none |
| `--cwd <path>` run against another project | ❌ missing | **add** |
| Built-in tools (read/write/edit/bash) | ✅ 138 tools (superset) | none |
| Durable JSONL sessions + resume + branching | ✅ JSONL + DAG branching | none |
| Slash commands (login, model, sessions, compaction, export, theme) | ⚠️ `/model`, `/compact`, `/export`, `/theme` yes; `/login` no | **add `/login`** |
| `AGENTS.md` project instructions | ✅ | none |
| `.tau/` project resources | ✅ `.rho/` | none |
| `.agents/` shared resources | ⚠️ partial (skills/providers/recipes) | **extend to instructions** |
| User skills, prompt templates, themes | ✅ | none |
| Context accounting + manual/auto compaction | ✅ | none |
| Provider-neutral output: text, JSON, transcript | ⚠️ text + json + stream-json; no transcript | **add transcript** |
| `~/.tau/catalog.toml` custom providers/models, no code changes | ❌ missing | **add `~/.rho/catalog.toml`** |
| `tau update` | ✅ `rho update` | none |
| Installers | ✅ `install.sh` | none |

## Work items

1. **`--cwd` global flag** — change directory before session start; applies to
   interactive, `-p`, and `exec`.
2. **`/login` slash command** — Tau's auth entry point; route to Rho's existing
   credential/config flow (no new secret handling).
3. **`transcript` output format** — add to `--output-format`; render the turn
   as a plain, timestamped transcript.
4. **User catalog `~/.rho/catalog.toml`** — let users add providers/models
   without code changes, merged over the eyrie catalog.
5. **`.agents/` instruction resources** — load `AGENTS.md`-style instructions
   from `.agents/` in addition to the existing skill/provider/recipe dirs.

## Verification (twice)

- Pass 1: gofumpt v0.10.0, goimports, `go build ./...`, `go vet ./...`,
  golangci-lint, targeted tests per touched package, boundary guards.
- Pass 2: independent re-check of each feature via CLI surface + targeted tests.

## Out of scope

- Python/Textual port. Rho stays Go.
- Removing Rho's larger feature surface.