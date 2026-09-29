# Releasing rho

Rho is pre-1.0. Releases are cut by hand: a maintainer merges a release PR
that sets `VERSION` and the `CHANGELOG.md` section, then pushes the matching
`v<VERSION>` tag. The tag triggers
[`.github/workflows/release.yml`](../.github/workflows/release.yml), which
builds, signs and publishes the GitHub release. There is no release-please or
other bot: nothing bumps `VERSION` or writes `CHANGELOG.md` automatically.

The first release under the `rho` name is **v0.3.0**.

## Tag protection

Only repository admins can create, move or delete release tags. The
repository ruleset **`protect-release-tags` (id 24053191)** is active for
tags matching `refs/tags/v*` and restricts creation, update, deletion and
non-fast-forward updates, with the repository admin role as the only bypass
actor. A push of a `v*` tag by anyone else is rejected, so a write-access
account cannot trigger a signed release. Inspect it with:

```bash
gh api repos/GrayCodeAI/rho/rulesets/24053191
```

The `main` branch is separately protected (required status checks, no force
pushes).

## Release checklist

1. **Release PR** (conventional title, e.g. `chore(release): v0.3.0`):
   - `VERSION` holds `X.Y.Z` (no leading `v`).
   - `CHANGELOG.md`: rename `## [Unreleased]` to `## [X.Y.Z] — YYYY-MM-DD`
     and add a new, empty `## [Unreleased]` above it. This section becomes the
     GitHub release notes verbatim.
   - `testdata/compatibility-matrix.json`: set the `stable` matrix to rho
     `X.Y.Z` and the flux version pinned in `go.mod`.
   - `README.md` install section: point the `install.sh` URL at the new tag.
   - Run locally:

     ```bash
     make release-check TAG=vX.Y.Z   # tag == v$(cat VERSION), CHANGELOG section present
     make release-snapshot           # goreleaser check + all archives into dist/, no publish/sign
     ```

2. **Merge** the PR and wait for CI on `main` to pass.
3. **Tag** the merge commit (admins only, see above):

   ```bash
   git fetch origin
   git tag -s vX.Y.Z -m "rho vX.Y.Z" <merge-commit-sha>
   git push origin vX.Y.Z
   ```

4. **Watch** the `release` workflow run to completion (next section).
5. **Verify** the published release as a user would (see
   [Verifying a release](#verifying-a-release)), including
   `curl -fsSL https://raw.githubusercontent.com/GrayCodeAI/rho/vX.Y.Z/install.sh | RHO_REQUIRE_COSIGN=1 sh`
   and `go install github.com/GrayCodeAI/rho/cmd/rho@vX.Y.Z`.

`make release` deliberately refuses to publish from a workstation: releases are
signed with the workflow's GitHub OIDC identity, which a local run cannot have.

## What the release workflow does

On a pushed `v*` tag, with `permissions: contents: write, id-token: write`:

1. `scripts/check-release-tag.sh` refuses the run unless the tag is exactly
   `v` + `VERSION` and `CHANGELOG.md` has a non-empty `## [<VERSION>]`
   section; that section is extracted as the release notes.
2. `scripts/check-no-replace-directives.sh` refuses local `replace`
   directives. The job sets `GOWORK=off`, so dependencies (including flux)
   come only from `go.mod`/`go.sum` via the public proxy.
3. syft (`anchore/sbom-action/download-syft`) and cosign
   (`sigstore/cosign-installer`) are installed; both actions are pinned by
   commit SHA.
4. GoReleaser v2.17.0 (`.goreleaser.yml`) builds and publishes:
   - `rho_<version>_<os>_<arch>.tar.gz` for linux and darwin and `.zip` for
     windows, each on amd64 and arm64 (six archives, `-trimpath`, `CGO_ENABLED=0`);
   - `rho_<version>_source.tar.gz`;
   - one SPDX SBOM per archive (`<archive>.spdx.sbom.json`);
   - `checksums.txt` (SHA-256 of every asset above);
   - `checksums.txt.sigstore.json`, a cosign keyless Sigstore bundle
     (certificate, signature and transparency-log proof) over `checksums.txt`.
5. The workflow re-downloads `checksums.txt` and the bundle from the published
   release and verifies them with the exact certificate identity, so a broken
   signature fails the run visibly.

## Verifying a release

```bash
tag=vX.Y.Z
gh release download "$tag" --repo GrayCodeAI/rho \
  --pattern checksums.txt --pattern checksums.txt.sigstore.json \
  --pattern "rho_${tag#v}_linux_amd64.tar.gz"
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/GrayCodeAI/rho/.github/workflows/release.yml@refs/tags/$tag" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
sha256sum --ignore-missing -c checksums.txt
```

`install.sh` performs the same checks: it always verifies the SHA-256, verifies
the bundle when cosign (v2.4 or later; v3 recommended) is installed, says so
plainly when it is not, and refuses to install without cosign when
`RHO_REQUIRE_COSIGN=1`.

## Distribution channels

| Channel | Status |
|---|---|
| GitHub release archives | From v0.3.0. |
| `install.sh` | From v0.3.0; refuses older (hawk-era) versions. |
| `go install github.com/GrayCodeAI/rho/cmd/rho@latest` | From v0.3.0. Until then `@main` works; `@latest` resolves to the hawk-era v0.2.0. |
| GitHub Action (`.github/actions/rho`) | From v0.3.0; installs through `install.sh` with cosign required. |
| systemd unit (`packaging/systemd/rho-daemon.service`) | Template for running `rho daemon`; see the troubleshooting guide. |
| Homebrew | **Not available.** Needs a `GrayCodeAI/homebrew-tap` repository, a fine-grained token with write access to it stored as a secret, and a `homebrew_casks` stanza in `.goreleaser.yml`. |
| npm | **Not available.** The scaffolding in `npm/` is marked private; see [`npm/README.md`](../npm/README.md). |
| Nix | **Not available.** The old flake could not evaluate and was removed; a new one needs a computed `vendorHash`, a committed `flake.lock` and `nix flake check` in CI. |

Add a channel to the README only after it has worked end to end for a real
release.

## Notes for v0.3.0 (first rho release)

- Tags `v0.1.0`, `v0.1.1` and `v0.2.0` were published under the previous
  module path `github.com/GrayCodeAI/hawk`; their assets are `hawk_*` archives
  containing a `hawk` binary. None of them is an ancestor of `main`, and
  `v0.1.1` has no GitHub release. The Go proxy keeps serving them, so v0.3.0's
  `go.mod` retracts `[v0.1.0, v0.2.0]`.
- Because no earlier tag is reachable from `main`, the release notes come only
  from the `CHANGELOG.md` section and the "Full changelog" compare link is
  omitted.
- v0.2.0 is the current "Latest" release. Once v0.3.0 is published it becomes
  Latest. Consider editing the v0.2.0 release notes to say it is the legacy
  hawk build and cannot be installed as rho.
