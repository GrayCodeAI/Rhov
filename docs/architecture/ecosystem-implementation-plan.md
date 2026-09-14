# Ecosystem Architecture Implementation Plan

This is the execution plan for the independent-repository architecture around
Rho. The plan treats `graycode-eco` as a parent folder only. It does not add a
root repository, root runtime module, or second product CLI.

## Target architecture

```text
SDKs (Sparrow / Robin / Wren)
              |
              v
       Rho CLI and daemon
              |
   +----------+----------+
   |          |          |
   v          v          v
Eyrie     Harrier     Shrike
models    memory      tokens
   |          |          |
   +----------+----------+
              |
       Eagle contracts
              |
   +----------+----------+
   |                     |
   v                     v
Falcon MCP kit     GrayCode Platform
                   HTTP / cloud plane

Swift, Kestrel, and Merlin are additional Rho capabilities.
Starling extends Rho through skills. Owl reads the canonical manifest.
```

## Invariants

1. `rho` is the only main product CLI and orchestration root.
2. `eagle` owns neutral cross-repository contracts.
3. Engines do not import Rho internals or peer engines.
4. SDKs use Rho's public HTTP/OpenAPI surface.
5. Starling uses Rho's skill/plugin surface.
6. Owl consumes `rho/ecosystem.yaml` as a read-only projection source.
7. No Go module imports `graycode-platform`.
8. Rho uses the platform only through optional authenticated runtime calls.
9. Every release must work with `GOWORK=off` and published module versions.

## Phase 1: Repository and naming baseline — complete

- Keep the repositories independent, each with its own Git history.
- Keep `graycode-eco` file-free except for checked-out repository directories.
- Use bird codenames consistently for directories, module paths, and repository
  names.
- Keep `rho/ecosystem.yaml` as the canonical inventory.
- Keep `owl/ecosystem.json` synchronized with the canonical inventory.
- Remove old repository directories and old module paths from active source.

Acceptance checks:

```text
bash rho/scripts/ecosystem-manifest.sh validate
bash owl/scripts/sync-ecosystem.sh
```

## Phase 2: Contract and dependency boundaries — complete locally

- Keep Eagle imports in Rho and engines limited to shared contracts.
- Keep Falcon imports limited to MCP-serving components.
- Keep Rho's Eyrie integration behind `eyrie/engine`.
- Keep Rho's SDKs outside the Go workspace and outside engine dependencies.
- Keep platform integration at the HTTP/Service Binding boundary.
- Keep graph and quality projections as explicit, reviewed Rho integration
  surfaces; do not spread their implementation types into unrelated packages.

Acceptance checks:

```text
bash rho/scripts/check-ecosystem-boundaries.sh
bash rho/scripts/check-support-repo-coupling.sh
bash rho/scripts/check-eyrie-engine-boundary.sh
bash rho/scripts/check-eyrie-client-imports.sh
bash rho/scripts/check-no-replace-directives.sh
```

## Phase 3: Published-module cutover — pending external release

The local Eyrie source already uses Eagle contracts. Rho's current published
Eyrie pseudo-version still declares the retired compatibility contract
transitively, so this phase requires publishing the compatible Eyrie revision.

1. Publish the current Eagle-compatible Eyrie revision.
2. Resolve its canonical Go pseudo-version from the published commit.
3. Update Rho's Eyrie requirement to that version.
4. Run `GOWORK=off go mod tidy` in Rho.
5. Confirm the retired compatibility module is absent from Rho's module graph.
6. Remove any obsolete transition excludes.
7. Run Rho's standalone release-parity checks.

Acceptance checks:

```text
GOWORK=off go mod tidy
GOWORK=off go list -m all
bash rho/scripts/check-module-release-parity.sh
GOWORK=off go test ./...
```

This phase is intentionally not simulated with a committed local `replace`.
The release graph must resolve from published module versions.

## Phase 4: SDK and platform integration — complete locally; release verify

- Keep Sparrow, Robin, and Wren aligned with Rho's public API snapshots.
- Keep SDKs independent from engines and Eagle implementation packages.
- Keep the Web app calling the BFF rather than reaching into Worker internals.
- Keep the BFF-to-Worker connection private through the configured Service
  Binding.
- Keep Worker persistence behind D1, Queue, and R2 boundaries.
- Keep the deployment identifier `graycode-cloud` inside the platform repo; it
  is not a repository name.

Acceptance checks:

```text
pnpm run typecheck:worker
pnpm run typecheck:bff
pnpm run test
pnpm run build
pnpm run lint
pnpm run format:check
```

## Phase 5: Continuous verification

- Run the manifest and boundary checks on every Rho ecosystem change.
- Run standalone module-mode tests before release; workspace tests are an
  additional integration pass.
- Regenerate Owl after every canonical manifest change.
- Reject new imports of Rho internals from sibling repositories.
- Reject new direct SDK-to-engine or engine-to-peer-engine dependencies.
- Record any intentional boundary exception in the architecture document and
  add a focused test or guard for it.

## Completion definition

The architecture is complete when:

- all local invariants and boundary checks pass;
- all repositories match the canonical manifest;
- Rho builds and tests with `GOWORK=off`;
- SDK and platform contract checks pass; and
- the published Eyrie revision no longer brings the retired compatibility
  contract into Rho's standalone module graph.

Current state: Phases 1, 2, and the local portion of Phase 4 are complete.
Phase 3 is the only remaining release-dependent item.
