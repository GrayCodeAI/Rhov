# npm packaging (not published)

**Rho is not available on npm.** Nothing is published under the `@graycodeai`
scope, no workflow publishes these packages, and every `package.json` in this
directory sets `"private": true` so `npm publish` refuses to run. Install Rho
as described in the [README](../README.md#install).

This directory is scaffolding for a future npm channel:

- `rho/` — the meta package `@graycodeai/rho`. Its `postinstall.js` copies the
  binary from the matching platform package into `~/.rho/bin` using the same
  versioned-file-plus-symlink layout as `install.sh`.
- `rho-<platform>-<arch>/` — one package per target (darwin, linux, win32 ×
  x64, arm64), each holding a brotli-compressed binary.
- `rho/scripts/assemble-platform-packages.js` — fills the platform packages
  from GoReleaser's `dist/` output and stamps every package, including the meta
  package's `optionalDependencies`, with the release version (the repository
  `VERSION` file, or `RHO_NPM_VERSION`).

To try the assembly locally without publishing:

```bash
goreleaser release --snapshot --clean --skip=publish,sign
cp -R npm VERSION "$(mktemp -d)"/   # work on a copy; assembly rewrites package.json files
# in the copy: cd npm/rho && DIST_ROOT=<repo>/dist node scripts/assemble-platform-packages.js
```

Before this channel can be offered, all of the following are needed:

1. An npm organization `graycodeai` owned by GrayCode, with npm trusted
   publishing (GitHub OIDC) configured for this repository's release workflow.
2. A release job that runs after GoReleaser: assemble the packages, publish the
   six platform packages, then the meta package, with `--provenance`.
3. A `THIRD_PARTY_NOTICES.md` (for example generated with `go-licenses`); the
   `files` lists reference it and the assemble script copies it when present.
4. Removing `"private": true` from the packages in the same change, and adding
   the npm command back to the README install section.
