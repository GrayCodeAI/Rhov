# Rho Provider Abstraction

## Goal

Rho must remain model agnostic.

That means Rho should support multiple providers without leaking provider-specific assumptions across the product.

## Design principle

Provider-specific code lives behind Flux's stable `flux/engine` host facade.
Rho is the face and composition root; Flux is the engine.

Rho decides:

- what capability the task needs
- the semantic intent (`fast`, `balanced`, `reasoning`, `economical`)
- whether an exact user-selected model may fall back

`flux` handles:

- capability-to-model resolution
- provider and deployment selection
- health-aware infrastructure routing and fallback
- request translation
- streaming normalization
- tool-call normalization
- retries and backoff
- provider capability differences
- credential storage, import, and sanitized provider state
- catalog/cache ownership and provider-scoped live discovery

## Required capabilities

- chat completion
- streaming
- tool calls
- model metadata
- token usage reporting
- error classification
- timeout/cancellation support

## Rho-facing abstraction

Rho depends on its small `ChatClient` product port and adapts it only to
`flux/engine`, never to a vendor-specific or lower-level Flux client.

Example concerns:

- `RunTurn`
- `StreamTurn`
- `SupportsTools`
- `SupportsVision`
- `SupportsLongContext`
- `SupportsJSONMode`

## Rules

- no direct vendor SDK imports in unrelated Rho packages
- no provider-specific branches inside review/verify logic
- no model-specific assumptions inside session persistence
- keep task-semantic policy inside Rho orchestration
- keep provider/deployment routing, health, retry, and fallback inside Flux
- Rho production integrations use `github.com/GrayCodeAI/flux/engine`
- direct imports of lower Flux packages are forbidden and CI-enforced
- custom gateway settings enter as `engine.Options.CustomGateways` and are
  isolated per Engine instance
- Rho-owned JSON, session, and conversation schemas are explicit projections;
  they never become aliases of engine DTOs
- local preflight readiness and remote live verification are distinct states

## Future extension

This design allows:

- local models
- hosted APIs
- custom gateways
- enterprise proxies
- model routing by capability or policy
