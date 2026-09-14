# Frontend contract

Rho has one agent brain and several frontends. They meet at a single typed
channel: `engine.StreamEvent`. This document defines that boundary so the TUI,
daemon, ACP server, and print mode stay swappable — the discipline Tau calls
"events are the contract."

## The contract

`engine.Session.Stream(ctx)` returns `<-chan engine.StreamEvent`. Every
frontend consumes that channel and renders it; nothing else crosses the
boundary.

```go
type StreamEvent struct {
    Type         string // content, thinking, tool_use, tool_result,
                        // usage, compact, compact_start, done, error
    Content      string
    ToolName     string
    ToolID       string
    ToolState    ToolState
    ToolReason   ToolTerminalReason
    Usage        *StreamUsage
    TokensBefore int
    TokensAfter  int
}
```

## Rules

1. **The engine emits, frontends consume.** A frontend must not call into
   engine internals to observe agent state; if it needs a new signal, add a
   `StreamEvent` type, not a getter.
2. **Events are the only cross-boundary type.** Frontends may read the
   public facade types (`engine.Session`, `engine.StreamEvent`,
   `engine.StreamUsage`) but must not import engine subpackages
   (`internal/engine/agent`, `internal/engine/code`, …).
3. **Terminal event is guaranteed.** Every turn ends with exactly one
   `EventDone` (or `done`), on success, error, and cancellation paths, so a
   consumer can always know a turn finished.
4. **Frontends own rendering.** Markdown, color, layout, and input handling
   live in the frontend. The engine never formats for a specific terminal.

## Consumers

| Frontend | Entry point | Role |
|---|---|---|
| TUI | `cmd/chat*.go` | Bubble Tea interactive interface |
| Print | `cmd/chat_print.go` | `-p` one-shot text/JSON output |
| Daemon | `internal/daemon` | HTTP + SSE over the same events |
| ACP | `internal/acp` | Editor (Zed) JSON-RPC bridge |

## Enforcement

`internal/testaudit` asserts that `cmd/` does not import engine subpackages
beyond the public facade. Run it with:

```bash
go test ./internal/testaudit -run TestFrontendContract -count=1
```