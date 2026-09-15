# Dynamic Model Discovery — Rho Developer Guide

## Ownership and flow

Rho is the product face; Flux is the provider engine. Rho owns the CLI/TUI,
model-picker presentation, user intent, and output compatibility. The
`flux/engine` facade alone owns provider registry details, credentials,
discovery, catalog/cache policy, model aliases, deployment routing, and chat
transport.

```text
provider APIs / remote catalog / local cache
                     |
                     v
            flux/engine.Engine
          catalog + credentials + routing
                     |
          stable host DTOs and methods
                     v
       Rho composition and presentation
          /config | models | conversation
```

Production Rho packages must not import Flux packages below
`github.com/GrayCodeAI/flux/engine`. Shell and AST guards enforce a
zero-exception boundary.

## Rho composition boundary

`internal/config` is Rho's control-plane composition root. It creates an
Flux engine and projects engine models into Rho UI and command contracts:

```go
models, err := config.ListEngineModels(ctx, "anthropic", false)
live, err := config.ListEngineModels(ctx, "anthropic", true)
public, err := config.ListPublicEngineModels(ctx, "xiaomi_mimo_payg")
```

Conversation construction goes through Rho's `internal/engine` adapter. The
adapter translates Rho-owned message, tool, usage, and stream DTOs to the
Flux engine facade; conversation history, WAL, resume, approvals, and tool
execution remain Rho-owned.

## Catalog and live discovery

The Flux engine combines its provider registry, provider-scoped live
discovery, and the cache at `~/.flux/model_catalog.json` by default. Override
the cache with `FLUX_MODEL_CATALOG_PATH`. Credential and state dependencies
are injected into each Engine instance, so discovery does not need hidden
process-global credentials.

Use the normal cache-backed path for repeatable UI and automation:

```bash
rho models list anthropic
rho models list anthropic --json
```

Use a provider-scoped live request when current connectivity and credentials
must be checked:

```bash
rho models list anthropic --live
rho models list anthropic --live --json
rho models list anthropic --live --raw
```

`rho preflight` reports **local readiness**: usable local state, a selected
model, and presence of the required stored credential. It is intentionally
cheap and does not prove that a remote provider accepts that credential.
Treat a successful provider-scoped `--live` request (or `/config` live
validation) as **live verified**.

## Stable command output

`rho models list --json` is a Rho-owned compatibility contract, not a direct
serialization of Flux's evolving `engine.Model` DTO. Its stable fields are:

```text
id, input_price_per_1m, output_price_per_1m, context_window, max_output,
server_tools, display_name, description, owner, live_metadata
```

New fields must be additive. `--raw` returns provider-native
`live_metadata` objects when available; for cache/public rows without native
metadata, it returns the stable Rho compatibility row instead of `null`.

## Custom gateways

Rho converts effective `custom_providers` settings into
`engine.Options.CustomGateways` at its composition root. Custom gateway
metadata is snapshotted per Engine instance. Do not register custom gateways
in Flux process-global state: tests, parallel sessions, and future multi-tenant
hosts must be isolated from one another.

## Adding or changing a provider

1. Implement registry, discovery, credentials, aliases, and transport behavior
   behind Flux's engine facade.
2. Add Flux tests for cache and live discovery, credential status, selection,
   and generation/streaming.
3. Commit and verify standalone Flux.
4. Advance Rho's `../flux` sibling checkout to that exact commit, then update
   Rho's module version when the Flux revision is published.
5. Verify both the workspace (`go.work`) and published-module
   (`GOWORK=off`) build modes.

Rho changes are needed only for a new product behavior or an additive
Rho-owned presentation field—not for provider-specific mechanics.

## Checks

```bash
rho models refresh
rho models status
rho preflight
make flux-engine-guard
go test ./cmd ./internal/config ./internal/engine -count=1
```
