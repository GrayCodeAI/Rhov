# Rho Examples

Rho is an AI coding agent that understands your codebase.

## Basic Usage

### Start a chat session

```bash
rho chat
> Explain how the authentication system works
```

### Generate code

```bash
rho "Add input validation to the user registration endpoint"
```

### Review changes

```bash
rho review
```

## Advanced Examples

### Use with skills

```bash
rho --skill code-review "Review my latest changes"
```

### Analyze codebase

```bash
rho analyze --depth full
```

### Fix issues

```bash
rho fix --auto
```

### Headless agent in CI

Copy [rho-ci-exec.yml](github/rho-ci-exec.yml) to run `rho exec --ephemeral --json`
on pull requests (summarize diff, risk list, or your own prompt). Pin your
rho install step and provider secrets before enabling the job.

### Report CI delivery context

Copy [rho-delivery-context.yml](github/rho-delivery-context.yml) to your
repository to report GitHub Actions runs to Rho Cloud. Create a dedicated,
revocable device token for CI and keep its endpoint, device ID, project ID, and
token in GitHub Actions secrets.

## MCP Integration

Rho can use MCP servers for extended capabilities:

```bash
# With harrier for persistent memory
harrier setup
rho chat

# With swift for session capture
swift start
rho "refactor the API layer"
swift stop
```

## Configuration

Create `.rho/config.json`:

```json
{
  "model": "claude-sonnet-4-6",
  "maxTokens": 8192,
  "temperature": 0.7
}
```

See the [main README](../README.md) for full documentation.
