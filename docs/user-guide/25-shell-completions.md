# Shell Completions

rho ships completion scripts for **bash**, **zsh**, **fish**, and **PowerShell**,
plus a machine-readable **JSON** spec for IDE integration.

## Quick Install

```bash
# Auto-install to the standard location for your shell and OS:
rho completion install bash
rho completion install zsh
rho completion install fish
```

## Manual Setup

### Bash

```bash
# Load for current session:
source <(rho completion bash)

# Persist (Linux):
rho completion bash > ~/.local/share/bash-completion/completions/rho

# Persist (macOS with Homebrew):
rho completion bash > /opt/homebrew/etc/bash_completion.d/rho
```

### Zsh

```bash
# Load for current session:
source <(rho completion zsh)

# Persist:
rho completion zsh > "${fpath[1]}/_rho"
```

### Fish

```bash
# Load for current session:
rho completion fish | source

# Persist:
rho completion fish > ~/.config/fish/completions/rho.fish
```

### PowerShell

```powershell
# Load for current session:
rho completion powershell | Out-String | Invoke-Expression

# Persist: add to your $PROFILE
rho completion powershell > rho.ps1
. ./rho.ps1
```

## JSON Spec (IDE Integration)

```bash
rho completion json
```

Prints a machine-readable command/flag spec that IDEs and editor plugins can
consume for inline completions without shell integration.

## Install Paths

`rho completion install` resolves the correct path automatically:

| Shell | Linux | macOS (Homebrew) |
|-------|-------|-------------------|
| bash | `~/.local/share/bash-completion/completions/rho` | `/opt/homebrew/etc/bash_completion.d/rho` |
| zsh | First `$fpath` entry (e.g. `/usr/local/share/zsh/site-functions/_rho`) | Same |
| fish | `~/.config/fish/completions/rho.fish` | Same |
