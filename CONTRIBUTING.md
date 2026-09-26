# Contributing to rho

Thanks for your interest! This guide covers the conventions used across the
GrayCodeAI repositories. The shared standards (versioning, release tooling, repo layout)
are defined in <https://github.com/GrayCodeAI/rho/blob/main/docs/versioning.md>.

## Quick start

1. Fork the repo and create a feature branch off `main`:
   ```bash
   git checkout -b feat/short-description
   ```
2. Make your changes in small, focused commits.
3. Run the full local check before pushing:
   ```bash
   make ci
   ```
4. Open a pull request. CI will re-run the same checks plus security
   scanning, race-detector tests, and (where applicable) integration tests.

## Build & test

This repo uses the standardised GrayCodeAI Makefile targets. Run `make help`
for the full list. The most common targets:

| Target              | What it does                                     |
| ------------------- | ------------------------------------------------ |
| `make build`        | Build the binary / verify the library compiles  |
| `make test`         | Run unit tests                                   |
| `make test-race`    | Run unit tests with the race detector            |
| `make cover`        | Generate a coverage report                       |
| `make lint`         | Run `golangci-lint` (pinned to CI's version)     |
| `make fmt`          | Format source files                              |
| `make vet`          | Run `go vet`                                     |
| `make security`     | Run `govulncheck`                                |
| `make ci`           | Run everything CI runs (the gate before pushing) |

Repo-specific dev targets:

| Target             | What it does                                                        |
| ------------------ | ------------------------------------------------------------------- |
| `make setup`       | Set up local development environment (go.work + external repos).     |
| `make boundaries`  | Alias for all boundary guards (matches `make boundaries` in engine repos). |
| `make test-10x`    | Run tests 10 times to surface flakes.                               |
| `make smoke`       | Quick build + doctor + ecosystem verification.                      |

## Commit message convention

We use [Conventional Commits](https://www.conventionalcommits.org/). Pull
request titles and commit subjects follow it so the history reads cleanly and
the release PR can group changes. There is no release bot: releases are cut by
hand (see [docs/RELEASING.md](docs/RELEASING.md)).

```
<type>(<optional scope>): <short summary>

<optional body>

<optional footer(s)>
```

**Types:**

- `feat:` — a new feature
- `fix:` — a bug fix
- `perf:` — performance improvement
- `refactor:` — code restructure with no behaviour change
- `docs:` — documentation only
- `test:` — adding or fixing tests
- `build:` — build system or dependencies
- `ci:` — CI configuration
- `chore:` — anything else
- `revert:` — reverts a previous commit

**Breaking changes:** add `!` after the type/scope or include `BREAKING
CHANGE:` in the footer, and call the change out in `CHANGELOG.md`.

Examples:

```
feat(client): add streaming retry with exponential backoff
fix: handle empty response body in chat handler
refactor!: rename ClientV1 to Client (BREAKING CHANGE)
```

**Co-authors:** do not add `Co-authored-by:` trailers. Commits should list
only the human author. Hooks are auto-installed by `make setup` via
[lefthook](https://github.com/evilmartians/lefthook). To re-install manually:

```bash
make hooks
```

**Cursor / IDE commits:** some editors inject `Co-authored-by: Cursor` via their
own git hooks. Use the clean commit helper to bypass IDE hooks while keeping
lefthook checks on normal commits:

```bash
./scripts/commit-clean.sh -m "fix(scope): your message"
```

**After a history rewrite:** reset stale local SHAs with:

```bash
./scripts/sync-clone.sh
```

## Commit signing

Signed commits are required in this repo.

- Git is configured for SSH commit signing.
- In sandboxed agent sessions, `git commit` can fail even when the signing key
  is unlocked because the sandbox cannot access the host `SSH_AUTH_SOCK`.
- When signing is required, run `git commit` outside the sandbox or through an
  unsandboxed/escalated execution path.

## Pull request checklist

Before requesting review:

- [ ] `make ci` passes locally.
- [ ] New behaviour has tests; bug fixes have a regression test.
- [ ] User-visible changes have an entry under `## [Unreleased]` in
      `CHANGELOG.md` (Keep a Changelog: Added / Changed / Fixed / Removed /
      Security).
- [ ] `VERSION` is left alone — it changes only in a release PR
      ([docs/RELEASING.md](docs/RELEASING.md)).
- [ ] Public API changes have updated doc comments.
- [ ] No secrets, API keys, or PII in code, comments, tests, or fixtures.

## Code review etiquette

- Reviewers focus on correctness, design, and tests; formatting is
  enforced by tooling, not humans.
- Authors respond to every comment (resolved, addressed, or politely
  declined with rationale) — no silent dismissals.
- Squash-merge by default; the PR title becomes the commit (so it must
  be a valid Conventional Commit message).
- One approving review from a CODEOWNERS-listed reviewer is required.

## Reporting bugs

Open an issue using the bug-report template. Include the `rho`
version (`rho --version` for binaries, `rho.Version` for
libraries — see this repo's `VERSION` file), reproduction steps, expected
behaviour, and actual behaviour.

## Reporting security issues

**Do not open a public issue.** See [SECURITY.md](./SECURITY.md) for
private reporting channels.

## License

By contributing, you agree that your contributions will be licensed under
the same license as this repo (see [LICENSE](./LICENSE)).
