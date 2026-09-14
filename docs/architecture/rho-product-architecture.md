# Rho Product Architecture

## Product statement

Rho is the model-agnostic main AI coding-agent CLI from GrayCodeAI.

`graycode-eco` is only the local parent folder used to develop the independent
repositories. It is not a monorepo or a runtime product.

Rho is the only primary product surface in the graycode-eco ecosystem. The support repos exist to power Rho, not to compete with it as standalone products.

For model execution specifically: **Rho is the face and composition layer;
Eyrie is the engine.** Rho owns the conversation and product experience while
the `eyrie/engine` facade owns the complete provider path from credential and
catalog state through model selection and normalized generation/streaming.

## Goals

- Keep Rho model agnostic.
- Keep Rho CLI-first and local-first.
- Keep provider integration pluggable.
- Keep the provider engine isolated behind a stable facade.
- Make review and verification first-class.
- Keep the design ready for a future hosted layer without making cloud a dependency.

## Target repo set

- `rho`
- `eyrie`
- `owl`
- `graycode-platform` (outside the Rho runtime module graph)

Directory names are authoritative for dependencies: `eyrie` is Eyrie. The
product label remains useful in CLI and user-facing documentation.

## Runtime architecture

```text
Users / SDKs / Skills
        |
        v
      RHO
        |
        +----------------------+
        |                      |
        v                      v
 core execution          embedded token engine
 eyrie/engine            internal/token
        |                      |
        +----------+-----------+
                   |
                   v
            internal/contracts
```

## Current vs proposed

Current implementation in the workspace:

```text
rho
  -> eyrie/engine
  -> internal/token (embedded)
  -> internal/contracts

x-> Rho internals from any external module
```

Proposed steady-state architecture:

```text
SDKs / Skills / future integrations
                |
                v
              Rho
                |
   +------------+------------+
   |                         |
   v                         v
 Eyrie/engine          internal/token
   |                         |
   +------------+------------+
                |
                v
        internal/contracts
```

This means:

- Rho is the product and orchestration boundary
- the provider engine stays behind its stable facade
- shared vocabulary lives in Rho's vendored contracts
- SDKs and community skills consume Rho, not the engine directly
- `graycode-platform` provides the optional hosted plane through HTTP and a
  private Service Binding; it is not imported by Rho

## Rho responsibilities

Rho owns:

- CLI entrypoints
- session lifecycle
- orchestration
- workflow control
- tool routing
- permission and policy enforcement
- user model preference and task-semantic model intent
- engine coordination
- public integration surfaces

Rho does not own:

- provider-specific implementation details
- engine-specific business logic
- future company-wide auth/billing/platform concerns

## Engine responsibilities

### `eyrie`
- stable host control and generation facade (`eyrie/engine`)
- credential storage and safe credential status
- catalog discovery and model metadata
- concrete provider/deployment selection
- provider adapters
- request/response normalization
- streaming
- retries/timeouts/fallbacks
- low-level provider registry and compatibility logic behind the engine facade

### `internal/token` (embedded)
- token budgeting and estimation
- context ranking, packing, and truncation
- compression, secret detection, and usage tracking
- model-ready context assembly inputs

## Primary runtime flow

1. User invokes `rho`.
2. Rho loads product settings, policy, and workspace state, then creates an
   Eyrie Engine with effective per-instance custom gateway settings.
3. Rho creates or resumes a session.
4. Rho assembles context through the embedded token engine.
5. Rho recalls relevant local memories.
6. Rho routes provider execution through `eyrie`.
7. Rho invokes tools and records graph observations.
8. Rho persists results and returns output to the user.

At step 6, Rho passes intent and Rho-owned conversation DTOs through its
adapter. Eyrie loads provider/catalog/credential state, resolves the gateway,
and returns normalized events. No production Rho package imports a lower
Eyrie package, and no Eyrie engine DTO is used as Rho's persistent or CLI
schema.

## Implementation phases

### Phase 1
- freeze architecture rules
- document repo roles
- define shared contracts inventory

### Phase 2
- add shared contracts
- move shared types out of Rho internals

Status:
- completed
- shared contracts now exist for `types`, `contracts/review`, `contracts/verify`, `tools`, `events`, and `policy`

### Phase 3
- remove engine imports of Rho internals
- remove engine-to-engine coupling

Status:
- completed for current workspace boundaries
- local/CI guards now block support-repo imports of `rho/internal/*`

### Phase 4
- harden orchestration boundaries in Rho
- formalize provider, review, and verify integration points

Status:
- completed for the local runtime boundary
- Rho owns runtime DTOs and review/verify product-boundary contracts
- Rho's `ChatClient` anti-corruption port translates only to `eyrie/engine`
- all lower-level Eyrie production imports are forbidden by shell guards and meta-audit tests

### Phase 5
- align SDKs and skills to Rho public interfaces only

Status:
- policy is now explicit and guarded in Rho docs
- broader non-Go consumer enforcement remains future work

## Done criteria

The architecture is in good shape when:

- `rho` is the only product surface
- the provider engine is consumed only through its stable facade
- shared types no longer live in Rho internals as a cross-repo API
- provider abstraction is stable
- review and verification are part of the standard runtime flow
- deprecated compatibility surfaces have a documented removal path and active guardrails
