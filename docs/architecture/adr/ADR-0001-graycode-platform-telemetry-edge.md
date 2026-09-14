# ADR-0001: Constrained telemetry edge from Rho to GrayCode Platform

- Status: Accepted
- Date: 2026-07-05
- Owners: Rho maintainers

## Context

`graycode-eco` is only a local parent folder and `rho` is the main CLI/product.
`graycode-platform` contains the optional GrayCode web application, browser
BFF, and Rho Cloud Worker. It is not a Rho runtime dependency.

Rho must remain fully functional as an OSS tool without the platform. At the
same time, Rho has an explicit cloud integration for usage and graph delivery,
so the boundary must be written down rather than implemented ad hoc.

## Decision

The compile-time rule is absolute:

- No Rho engine, SDK, skill, or foundation may import `graycode-platform` code
  or generated clients.
- No Go module in the ecosystem may depend on the platform repository.
- Engines never report directly; Rho owns optional platform communication.

A narrow runtime exception is sanctioned:

- Rho may send usage/activity telemetry to the deployed
  `graycode-platform/apps/worker` service (`graycode-cloud`) over HTTP.
- Rho may explicitly synchronize a prepared graph through the same service.
- Browser requests reach the Worker through `graycode-platform/apps/bff` and a
  private `RhoCloudService` binding; browser cookies and service tokens do not
  cross that boundary.

The runtime edge must satisfy these conditions:

1. **Opt-in.** Cloud connection requires explicit user configuration or device
   authentication; it is disabled by default.
2. **Fail-open for automatic delivery.** Endpoint, auth, and schema failures
   must never block or alter local Rho execution.
3. **HTTP-only.** No shared platform package, vendored client, or platform
   dependency is added to `go.mod`.
4. **Rho-owned.** Engines may be named in an event, but only Rho sends it.
5. **Bounded and privacy-safe.** Raw prompts, source, credentials, and
   transcript content are not sent by default; graph uploads are normalized and
   size-limited before transmission.

## Consequences

- `graycode-platform` remains outside the Rho Go workspace and runtime graph.
- The Worker deployment may be called `graycode-cloud` without creating a
  separate `graycode-cloud` repository.
- Automatic usage delivery is best-effort; explicit graph sync reports errors
  to the user because it is a user-requested operation.
- Any broader edge—engines reporting directly, Rho importing platform code,
  or Rho reading platform state implicitly—requires a new ADR.
