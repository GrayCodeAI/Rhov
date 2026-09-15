#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if command -v rg >/dev/null 2>&1; then
  flux_imports="$(
    rg -n '"github\.com/GrayCodeAI/flux/[^\"]+"' \
      --glob '*.go' --glob '!*_test.go' . || true
  )"
else
  flux_imports="$(
    grep -RInE --include='*.go' --exclude='*_test.go' \
      '"github\.com/GrayCodeAI/flux/[^\"]+"' . || true
  )"
fi
# Host contract surface is exactly four packages: engine (facade), llm (DTOs
# and the Provider port), graph (portable graph vocabulary), tools (tool-call
# contracts). See flux/README.md "Ecosystem Boundaries".
violations="$(printf '%s\n' "$flux_imports" | grep -vE '"github\.com/GrayCodeAI/flux/(engine|llm|graph|tools)(/|\")' || true)"

if [[ -n "$violations" ]]; then
  echo "direct production imports below the flux/engine facade found:"
  echo "$violations"
  echo
  echo "route every Rho production integration through github.com/GrayCodeAI/flux/engine"
  exit 1
fi

if command -v rg >/dev/null 2>&1; then
  credential_symbols="$(rg -n '\b(apiKeys|SetAPIKey|SetAPIKeys)\b' internal/engine --glob '*.go' --glob '!*_test.go' || true)"
else
  credential_symbols="$(grep -RInE --include='*.go' --exclude='*_test.go' '(^|[^[:alnum:]_])(apiKeys|SetAPIKey|SetAPIKeys)([^[:alnum:]_]|$)' internal/engine || true)"
fi
if [[ -n "$credential_symbols" ]]; then
	printf '%s\n' "$credential_symbols"
  echo
  echo "provider credentials must not enter Rho's agent/session layer"
  exit 1
fi

echo "flux engine boundary passed (zero lower-level production imports)"
