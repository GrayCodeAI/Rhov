#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ECO_DIR="$(cd "${ROOT_DIR}/.." && pwd)"
# The product repo may still be checked out under its pre-rename directory
# name (e.g. `hawk`) while ecosystem.yaml already lists `rho`. Derive the
# self directory from the checkout itself so the workspace generates during
# and after the rename instead of hardcoding one name.
SELF_DIR="$(basename "${ROOT_DIR}")"
# macOS checkouts are case-insensitive, so `Rho` and the manifest's `rho`
# name the same directory. Compare case-insensitively to avoid adding the
# self module twice.
SELF_DIR_LC="$(printf '%s' "${SELF_DIR}" | tr '[:upper:]' '[:lower:]')"

"${ROOT_DIR}/scripts/ecosystem-manifest.sh" validate

rm -f "${ECO_DIR}/go.work" "${ECO_DIR}/go.work.sum"
(
  cd "${ECO_DIR}"
  go work init "./${SELF_DIR}"
  go_version="$(awk '$1 == "go" { print $2; exit }' "${ROOT_DIR}/go.mod")"
  go work edit -go="${go_version}"
  while IFS= read -r repo; do
    repo_lc="$(printf '%s' "${repo}" | tr '[:upper:]' '[:lower:]')"
    [[ "${repo_lc}" == "${SELF_DIR_LC}" ]] && continue
    if [[ ! -f "${repo}/go.mod" ]]; then
      echo "WARNING: ${repo} is not checked out; skipping workspace entry"
      continue
    fi
    go work use "./${repo}"
    module="$(awk '$1 == "module" { print $2; exit }' "${repo}/go.mod")"
    while IFS= read -r version; do
      [[ -n "${version}" ]] && go work edit -replace="${module}@${version}=./${repo}"
    done < <(
      {
        while IFS= read -r consumer; do
          [[ -f "${consumer}/go.mod" ]] || continue
          awk -v wanted="${module}" '
            /^(replace|exclude)[[:space:]]*\($/ { skip = 1; next }
            skip && /^\)/                       { skip = 0; next }
            skip                                { next }
            $1 == "replace" || $1 == "exclude"  { next }
            {
              for (i = 1; i < NF; i++) {
                if ($i == wanted && $(i + 1) ~ /^v[0-9]/) print $(i + 1)
              }
            }
          ' "${consumer}/go.mod"
        done < <("${ROOT_DIR}/scripts/ecosystem-manifest.sh" list workspace)
      } | sort -u
    )
  done < <("${ROOT_DIR}/scripts/ecosystem-manifest.sh" list workspace)
  go work sync
)

echo "workspace generated at ${ECO_DIR}/go.work"
