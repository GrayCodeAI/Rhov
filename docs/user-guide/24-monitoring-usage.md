# Monitoring and Usage

Rho tracks token usage, costs, and session activity for monitoring and debugging.

---

## Usage Tracking

Token usage is tracked per session and aggregated.

### View Usage

```bash
rho usage
rho doctor    # Full health report
```

Usage shows:
- Input/output tokens per model
- Total cost estimate
- Session count

---

## Telemetry

Rho can send anonymous usage telemetry.

### Enable/Disable

```json
// ~/.rho/settings.json
{
  "telemetry": {
    "enabled": false
  }
}
```

Or:

```bash
rho --telemetry    # Enable
haw --no-telemetry  # Disable
```

---

## OpenTelemetry Export

Export usage to your own OTEL collector:

```json
{
  "telemetry": {
    "otel_enabled": true,
    "otel_endpoint": "https://collector.example.com:4318"
  }
}
```

Set `OTEL_EXPORTER_OTLP_HEADERS` for authentication.

---

## Session Metrics

Per-session metrics include:
- Turn count
- Tool call count
- Token totals
- Duration

Accessible via `/ecosystem` in TUI.

---

## Cloud Integration

Rho Cloud provides managed usage tracking:

- Organization-level metrics
- Budget alerts
- Entitlement management

See graycode-cloud documentation for details.

---

© 2026 GrayCode AI. All rights reserved.