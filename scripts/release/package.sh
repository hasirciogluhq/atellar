#!/usr/bin/env bash
set -euo pipefail

# Build release tarball for maintainers (CI / local).
# Usage: ./scripts/release/package.sh v0.1.0

VERSION="${1:-}"
[[ -n "${VERSION}" ]] || { echo "usage: $0 <version> (e.g. v0.1.0)" >&2; exit 1; }

VERSION_TAG="${VERSION#v}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DIST="${REPO_ROOT}/dist/atellar_${VERSION_TAG}_linux_amd64"

rm -rf "${DIST}"
mkdir -p "${DIST}/bin" "${DIST}/migrations"

echo "building linux/amd64 binaries..."
(
  cd "${REPO_ROOT}"
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${DIST}/bin/atellar-api" ./cmd/api
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${DIST}/bin/atellar-agent" ./cmd/agent
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${DIST}/bin/atelctl" ./cmd/atelctl
)

cp -a "${REPO_ROOT}/internal/db/migrations/." "${DIST}/migrations/"
cp "${REPO_ROOT}/scripts/release/install.sh" "${DIST}/install.sh"
cp "${REPO_ROOT}/scripts/release/uninstall.sh" "${DIST}/uninstall.sh"
chmod +x "${DIST}/install.sh" "${DIST}/uninstall.sh"
echo "${VERSION}" >"${DIST}/VERSION"

OUT="${REPO_ROOT}/dist/atellar_${VERSION_TAG}_linux_amd64.tar.gz"
tar -czf "${OUT}" -C "${REPO_ROOT}/dist" "atellar_${VERSION_TAG}_linux_amd64"

echo "created ${OUT}"
echo "upload to GitHub release v${VERSION_TAG} with install.sh + tarball"
