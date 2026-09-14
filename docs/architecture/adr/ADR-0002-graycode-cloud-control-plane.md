# ADR-0002: Rho Cloud control plane inside GrayCode Platform

- Status: Accepted
- Date: 2026-07-10
- Owners: Rho maintainers

## Decision

`graycode-platform` is the separate web/platform repository. It contains two
Cloudflare Worker applications:

- `apps/bff` owns browser identity and the authenticated browser gateway;
- `apps/worker`, deployed as `graycode-cloud`, owns the Rho-specific control
  plane and source of truth for organizations, teams, projects, devices,
  sessions, usage, policies, entitlements, billing, audit, and graph records.

Rho remains local-first. It may send aggregate usage or explicitly requested
graph data to Rho Cloud only after the user connects a device. Engines never
import GrayCode Platform or Rho Cloud code; Rho uses its isolated HTTP cloud
adapter.

The browser reaches Rho Cloud only through the BFF's private
`RhoCloudService` Service Binding. The BFF resolves the authenticated
principal; the Worker injects its cloud-only service credential. The browser
never receives that credential.

## Authentication

Interactive CLI authentication uses a browser device-authorization flow:

```text
rho cloud login
  -> GrayCode Platform BFF/browser authentication
  -> user approves organization/project
  -> Rho Cloud Worker issues one project-scoped device token
  -> Rho stores it in the OS secure store
```

Automation uses project-scoped service accounts or API keys. Human credentials
must not be shared with CI systems.

## Consequences

- GrayCode product activity remains separate from Rho's authoritative usage
  and billing ledger.
- Usage events are versioned and idempotent, with token dimensions and session
  attribution.
- The Worker uses D1 for control-plane state, Queue plus an outbox for durable
  usage delivery, and R2 for large redacted exports.
- Full traces and large exports are opt-in, redacted, encrypted where needed,
  and retention-controlled.
- The no-compile-time-dependency rule in ADR-0001 remains unchanged.
