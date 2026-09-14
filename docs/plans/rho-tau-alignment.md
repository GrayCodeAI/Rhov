# Plan: Rho — Tau-style TUI restyle + architecture tightening

Status: in progress
Branch: `feat/rename-rho`

## Context

Rho (formerly Hawk) is a Go terminal coding agent (Bubble Tea + Lip Gloss,
~17.7k LOC TUI across 107 files, 138 tools, 64 commands). Tau is a small
Python/Textual teaching agent with a strict 3-layer split
(`tau_ai → tau_agent → tau_coding`).

"Make Rho like Tau" cannot mean a language port or a Textual clone. This plan
does the achievable parts:

1. **Tau-style visual restyle** — adopt Tau's restrained, low-chrome look in
   Rho's existing Bubble Tea TUI.
2. **Tau-style architecture discipline** — make `engine.StreamEvent` the hard
   frontend contract; document and enforce the boundary.
3. **Verify twice** — targeted tests + lint, then a second independent pass.

## Phase 0 — Rename completion (done)

- [x] Content rename hawk → rho (0 refs remain except intentional shims)
- [x] File/dir renames (`cmd/rho`, `internal/rhoerr`, npm packages)
- [x] Module path `github.com/GrayCodeAI/rho`
- [x] Compatibility shims: `env.GetenvRho`, `storage.resolveAppDir`,
      legacy `HAWK_*_DIR` overrides
- [x] Committed + pushed to `feat/rename-rho`

## Phase 1 — Tau-style theme (visual)

Tau's palette is a restrained dark theme: near-black panel, single muted
accent, generous dim text, no heavy borders.

- [ ] Add a `tau` palette to `internal/theme/theme_palettes.go`
      (panel `#0d1117`, ink `#e6edf3`, muted `#8b949e`, accent `#7aa2f7`,
      subtle line `#21262d`).
- [ ] Register it in `themeRegistry`; make it selectable via `/theme tau`.
- [ ] Reduce chrome: prefer single-line separators over box borders where the
      theme is `tau`.
- [ ] Keep every existing palette intact (no regressions).

## Phase 2 — Tau-style layout (visual)

- [ ] Flatten the footer: one status line + one input line (Tau shows a
      minimal bottom bar, not a multi-row panel).
- [ ] Dim the input border; show the mode as a short prefix rather than a
      separate row.
- [ ] Tighten the welcome block: wordmark + one hint line.
- [ ] Preserve all existing information (model, ctx, cost) in the compact row.

## Phase 3 — Architecture tightening (contract)

- [ ] Document the frontend contract in `docs/architecture/frontend-contract.md`:
      `engine.StreamEvent` is the only channel; TUI/daemon/ACP/print are
      consumers and must not reach into engine internals.
- [ ] Add a testaudit guard asserting `cmd/` TUI files do not import
      `internal/engine/*` subpackages beyond the public facade types.

## Phase 4 — Verification (twice)

- [ ] Pass 1: `gofumpt v0.10.0`, `goimports`, `go build ./...`, `go vet ./...`,
      `golangci-lint`, targeted tests for touched packages, boundary guards.
- [ ] Pass 2: independent re-run of the same, plus a fresh-eyes grep for
      stale references and a manual TUI render check.

## Out of scope (explicitly)

- Rewriting Rho in Python / cloning Textual.
- Deleting Rho's tool/command surface (separate decision).
- Changing the daemon/ACP wire protocols.