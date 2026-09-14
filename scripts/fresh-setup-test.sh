#!/usr/bin/env bash
# Fresh first-run /config test — build rho and optional isolated ~/.rho.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "==> Ensuring ecosystem repos are present (make setup)..."
make setup

echo ""
echo "==> Building rho..."
go build -o rho ./cmd/rho/
echo "    $(./rho --version 2>/dev/null || echo built)"

echo ""
echo "==> Credential status (macOS Keychain / secret service):"
./rho credentials status || true

echo ""
echo "==> Optional: isolated config dir (sessions/settings separate from ~/.rho)"
ISOLATED="${RHO_FRESH_CONFIG_DIR:-$(mktemp -d)/rho-fresh}"
mkdir -p "$ISOLATED"
rm -f "$ISOLATED/learned_credential_prefixes.json"
rm -f "$ISOLATED/settings.json"
echo "    RHO_CONFIG_DIR=$ISOLATED"

echo ""
echo "To match first-run Setup (no API key in Keys tab):"
echo "  - Remove stored keys: ./rho credentials remove <provider>   (e.g. anthropic, openai)"
echo "  - This script already clears the isolated settings.json so model selection starts fresh"
echo ""
echo "Run TUI (from $ROOT):"
echo "  export RHO_CONFIG_DIR=\"$ISOLATED\""
echo "  ./rho"
echo ""
echo "In Setup: Keys → Add key · <Gateway> → paste → one probe → Models."
echo "After setup: ./rho path  and  ./rho preflight"
