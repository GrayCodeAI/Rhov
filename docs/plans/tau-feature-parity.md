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
| `--cwd <path>` run against another project | ✅ `--cwd` | none |
| Built-in tools (read/write/edit/bash) | ✅ 138 tools (superset) | none |
| Durable JSONL sessions + resume + branching | ✅ JSONL + DAG branching | none |
| Slash commands (login, model, sessions, compaction, export, theme) | ✅ `/login` added | none |
| `AGENTS.md` project instructions | ✅ | none |
| `.tau/` project resources | ✅ `.rho/` | none |
| `.agents/` shared resources | ✅ skills/providers/recipes/hooks/rules/workflows + `.agents/AGENTS.md` instructions | none |
| User skills, prompt templates, themes | ✅ | none |
| Context accounting + manual/auto compaction | ✅ | none |
| Provider-neutral output: text, JSON, transcript | ✅ text + json + stream-json + transcript | none |
| `~/.tau/catalog.toml` custom providers/models, no code changes | ❌ missing | **flux-side** (see below) |
| `tau update` | ✅ `rho update` | none |
| Installers | ✅ `install.sh` | none |

## Work items

1. ~~**`--cwd` global flag**~~ — done (`cmd/root.go`, applied to interactive, `-p`, and `exec`).
2. ~~**`/login` slash command**~~ — done (`cmd/chat_subcommand_login.go`).
3. ~~**`transcript` output format**~~ — done (`--output-format transcript`).
4. **User catalog `~/.rho/catalog.toml`** — NOT implemented. The model catalog
   is owned by the flux engine (`github.com/GrayCodeAI/flux/catalog`), whose
   `LoadCatalogOptions` has no user-catalog override. Per `AGENTS.md`
   (provider/catalog ownership lives in flux), this must be added in flux and
   surfaced through the engine facade before rho can consume it.
5. ~~**`.agents/` instruction resources**~~ — done (`.agents/AGENTS.md` is in
   the instruction search path in `internal/config/config.go`).

## Verification (twice)

- Pass 1: gofumpt v0.10.0, goimports, `go build ./...`, `go vet ./...`,
  golangci-lint, targeted tests per touched package, boundary guards.
- Pass 2: independent re-check of each feature via CLI surface + targeted tests.

## Out of scope

- Python/Textual port. Rho stays Go.
- Removing Rho's larger feature surface.