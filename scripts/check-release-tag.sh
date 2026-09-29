#!/usr/bin/env bash
# Release guard: refuse to release a tag that disagrees with the repository.
#
# Usage: scripts/check-release-tag.sh [--notes <file>] <tag>   (e.g. v0.3.0)
#
# Checks, relative to the repository root (or RELEASE_CHECK_ROOT when set):
#   1. VERSION holds a SemVer 2.0.0 version (no leading "v").
#   2. <tag> is exactly "v" + VERSION.
#   3. CHANGELOG.md has a non-empty "## [<VERSION>]" section.
# With --notes, that section's body is written to <file> for use as the
# GitHub release notes.
#
# Run by .github/workflows/release.yml before GoReleaser and by
# `make release-check`. See docs/RELEASING.md.
set -euo pipefail

notes_file=""
if [ "${1:-}" = "--notes" ]; then
  if [ "$#" -lt 2 ] || [ -z "$2" ]; then
    echo "usage: $0 [--notes <file>] <tag>" >&2
    exit 2
  fi
  notes_file="$2"
  shift 2
fi
if [ "$#" -ne 1 ] || [ -z "$1" ]; then
  echo "usage: $0 [--notes <file>] <tag>   (for example: $0 v0.3.0)" >&2
  exit 2
fi
tag="$1"

root="${RELEASE_CHECK_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"

if [ ! -f "${root}/VERSION" ]; then
  echo "ERROR: ${root}/VERSION not found" >&2
  exit 1
fi
version="$(tr -d '[:space:]' < "${root}/VERSION")"

semver_re='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'
if ! [[ "${version}" =~ ${semver_re} ]]; then
  echo "ERROR: VERSION '${version}' is not a SemVer version like 1.2.3 (no leading 'v')" >&2
  exit 1
fi

if [ "${tag}" != "v${version}" ]; then
  echo "ERROR: tag '${tag}' does not match VERSION '${version}' (expected tag 'v${version}')." >&2
  echo "       Bump VERSION in a release PR, merge it, then tag that commit." >&2
  exit 1
fi

if [ ! -f "${root}/CHANGELOG.md" ]; then
  echo "ERROR: ${root}/CHANGELOG.md not found" >&2
  exit 1
fi
# Print the body of the "## [<version>]" section: literal prefix match on the
# heading (so '.' is not a wildcard), up to the next "## " heading.
section="$(awk -v want="## [${version}]" '
  index($0, want) == 1 { in_section = 1; next }
  in_section && /^## / { exit }
  in_section { print }
' "${root}/CHANGELOG.md")"
if [ -z "$(printf '%s' "${section}" | tr -d '[:space:]')" ]; then
  echo "ERROR: CHANGELOG.md has no non-empty '## [${version}]' section." >&2
  echo "       Move the [Unreleased] notes under '## [${version}] — YYYY-MM-DD' in the release PR." >&2
  exit 1
fi
if [ -n "${notes_file}" ]; then
  printf '%s\n' "${section}" > "${notes_file}"
fi

echo "OK: tag ${tag} matches VERSION ${version} and CHANGELOG.md has a [${version}] section."
