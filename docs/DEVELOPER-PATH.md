# Rho Developer Path

This guide explains what `rho path` checks and how to get a fresh developer machine ready to use Rho safely.

## What "developer path" means

For Rho, the developer path is the minimum local setup required to chat, edit code, and keep credentials off disk:

- A provider credential stored in the OS secret store
- A model selected in Rho settings
- A local model catalog available through eyrie
- No plaintext API keys left in Eyrie's configured `provider.json` or legacy env files
- Safe defaults for Bash execution and filesystem access
- Optional but healthy local memory and token-pipeline state

Run the report at any time:

```bash
rho path
rho path --strict
rho doctor
rho preflight
```

## Setup checklist

### 1. Build and workspace setup

If you are contributing from source, clone Rho as the main CLI. The support
repositories are independent sibling checkouts when you need the full local
workspace; they are not nested under Rho:

```bash
mkdir graycode-eco && cd graycode-eco
git clone https://github.com/GrayCodeAI/rho
git clone https://github.com/GrayCodeAI/eyrie
cd rho
make setup
go build -o rho ./cmd/rho
```

`make setup` validates the canonical repository manifest and regenerates the
parent `../go.work` from the local Go repositories marked `workspace: true`.
Rho can also be built as a standalone checkout with `GOWORK=off go build ./cmd/rho`;
the sibling workspace is only required for cross-repository development and
boundary checks.

### 2. Configure credentials

Start Rho and use `/config` to paste an API key or configure a local provider like Ollama.

Rho stores credentials in the macOS Keychain or Linux secret store. It should not rely on shell env vars, `.env`, or plaintext config files for provider secrets.

Useful checks:

```bash
rho credentials status
rho preflight
```

`rho preflight` is a local-ready check; it does not contact a provider. To
live-verify the selected provider credential and connectivity, use `/config`
validation or `rho models list <provider> --live`.

### 3. Select a model

Pick a model in `/config`. Rho stores the selected model in settings and uses eyrie for provider routing and catalog resolution.

If the catalog is missing or empty:

```bash
rho models refresh
```

## Security checks

`rho path` treats these as important security conditions:

- Eyrie's resolved `provider.json` must not contain secret fields
- sensitive files like provider config and SSH paths should be blocked from agent reads

Eyrie resolves provider state from `EYRIE_CONFIG_DIR` first, then the
platform user-config directory.
Rho protects that resolved path even when it is customized or symlinked.

If Rho detects secret fields in `provider.json`, back up the file, remove those fields manually, and save your keys again through `/config`.

Read the full credential and isolation model in [SECURITY-DEVELOPER.md](./SECURITY-DEVELOPER.md).

## Execution model

Rho executes agent commands directly on the host. No Docker daemon or
container runtime is required, and there is no container fallback to configure.

## Ecosystem checks

`rho path` also verifies the provider layer behind Rho:

- `eyrie` for provider routing and local preflight readiness
- the embedded token pipeline for estimation and compression

If you want the broader status summary:

```bash
rho ecosystem
rho doctor
```

## Typical recovery path

If `rho path` says you are not ready, this is the intended order:

1. Run `rho`
2. Open `/config`
3. Paste an API key or configure Ollama
4. Pick a model
5. Re-run `rho preflight`
6. Re-run `rho path`

If security items still fail, fix those before treating the machine as ready.
