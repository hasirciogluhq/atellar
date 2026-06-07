#!/usr/bin/env bash
set -euo pipefail

# Build universal linux release tarball (amd64 + arm64).
# Usage: ./scripts/release/build-all.sh v0.1.0

VERSION="${1:-}"
[[ -n "${VERSION}" ]] || { echo "usage: $0 <version> (e.g. v0.1.0)" >&2; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
"${SCRIPT_DIR}/package.sh" "${VERSION}"
