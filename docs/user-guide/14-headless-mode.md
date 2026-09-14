# Headless Mode and Scripting

Headless mode runs Rho non-interactively from the command line. It accepts a prompt, executes tools, and returns results — ideal for automation and CI/CD.

---

## Basic Usage

Pass a prompt to run headless:

```bash
rho -p "Your prompt here"
```

Rho processes the prompt and prints the result to stdout.

---

## Output Formats

### plain (default)

Human-readable text:

```bash
rho -p "Summarize this codebase"
```

### json

Single JSON object after completion:

```bash
rho -p "Summarize this codebase" --output-format json | jq -r '.response'
```

Output includes:
- `response` — Response content
- `exit_code` — 0 for success, non-zero for a failed run
- `session_id` — Session ID for resuming

### stream-json

NDJSON events in real time:

```bash
rho -p "Summarize" --output-format stream-json | jq -r 'select(.type=="content") | .content'
```

Event types:
- `content` — Response chunk
- `tool_use` / `tool_result` — Tool lifecycle events
- `done` — Final event with metadata
- `error` — Error occurred

---

## Session Management

### Named Session

Each `rho -p` creates a fresh session by default. To continue a session:

```bash
# Get session ID
rho -p "Initial prompt" --output-format json | jq -r '.sessionId'

# Resume the session
rho -p "Follow-up" --resume <session-id>
```

### Continue Most Recent

```bash
rho -p "Continue" --continue
```

---

## Tool Filtering

Restrict available tools:

```bash
# Allow only read tools
rho -p "Explain this" --tools "Read,Grep,LS"

# Deny specific tools
rho -p "Review" --disallowed-tools "Bash,WebSearch"
```

---

## Permission Rules

Control tool permissions:

```bash
# Allow shell commands through the explicit tool policy flag
rho -p "Build" --allowed-tools "Bash(git:*) Bash(npm:*)"

# Deny dangerous commands
rho -p "Clean" --disallowed-tools "Bash(rm:*) Bash(sudo:*)"
```

---

## Auto-Approve Mode

Use `--auto` for fully automated runs:

```bash
rho -p "Format all files" --dangerously-skip-permissions
rho exec --auto full "Add error handling"
```

**Warning:** This grants full autonomy. Use only in trusted environments.

---

## CI/CD Integration

### Pre-Commit Hook

```bash
#!/bin/bash
rho -p "Review staged changes for bugs. Reply OK if fine." \
  --dangerously-skip-permissions --output-format json | jq -r '.response' | grep -q "^OK" || exit 1
```

### Code Review

```bash
rho -p "Review PR for security issues" \
  --output-format json --dangerously-skip-permissions | jq -r '.response' > review.md
```

### Batch Processing

```bash
for file in src/*.go; do
  rho -p "Format $file" --auto
done
```

---

## Environment Variables

```bash
export XAI_API_KEY="xai-..."     # API key
export RHO_HOME="/path"          # Custom config location
export RHO_LOG_FILE="/tmp/rho.log"  # Log file
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error |
| `130` | Interrupted (SIGINT) |
| `143` | Terminated (SIGTERM) |

---

## Where to Go Next

| Document | What You Will Learn |
|----------|-------------------|
| [Agent Mode](15-agent-mode.md) | Agent configuration |
| [Subagents](16-subagents.md) | Parallel agent sessions |

---

© 2026 GrayCode AI. All rights reserved.
