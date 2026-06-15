#!/usr/bin/env bash
set -euo pipefail

# Build a universal linux release tarball (amd64 + arm64).
# Usage: ./scripts/release/package.sh v0.1.0

VERSION="${1:-}"
[[ -n "${VERSION}" ]] || { echo "usage: $0 <version> (e.g. v0.1.0)" >&2; exit 1; }

VERSION_TAG="${VERSION#v}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT_DIR="${REPO_ROOT}/scripts/release"
PKG_NAME="atellar_${VERSION_TAG}_linux"
DIST="${REPO_ROOT}/dist/${PKG_NAME}"
ASSETS_DIR="${REPO_ROOT}/dist/release-assets"

PLATFORMS=(
  "linux amd64"
  "linux arm64"
)

bake_release_script() {
  local src="$1"
  local dst="$2"
  sed "s/^RELEASE_VERSION=\"\"$/RELEASE_VERSION=\"${VERSION_TAG}\"/" "${src}" >"${dst}"
  chmod +x "${dst}"
}

rm -rf "${DIST}" "${ASSETS_DIR}"
mkdir -p "${DIST}/migrations" "${ASSETS_DIR}"

build_binaries() {
  local goos="$1"
  local goarch="$2"
  local out_dir="${DIST}/${goarch}/bin"

  mkdir -p "${out_dir}"
  echo "building ${goos}/${goarch}..."
  (
    cd "${REPO_ROOT}"
    export GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0
    go build -trimpath -ldflags="-s -w" -o "${out_dir}/atellar-api" ./cmd/api
    go build -trimpath -ldflags="-s -w" -o "${out_dir}/atelagent" ./cmd/atelagent
    go build -trimpath -ldflags="-s -w" -o "${out_dir}/ateladm" ./cmd/ateladm
    go build -trimpath -ldflags="-s -w" -o "${out_dir}/atelctl" ./cmd/atelctl
  )
}

for entry in "${PLATFORMS[@]}"; do
  read -r goos goarch <<<"${entry}"
  build_binaries "${goos}" "${goarch}"
done

cp -a "${REPO_ROOT}/internal/db/migrations/." "${DIST}/migrations/"
bake_release_script "${SCRIPT_DIR}/install.sh" "${DIST}/install.sh"
bake_release_script "${SCRIPT_DIR}/uninstall.sh" "${DIST}/uninstall.sh"
cp "${DIST}/install.sh" "${ASSETS_DIR}/install.sh"
cp "${DIST}/uninstall.sh" "${ASSETS_DIR}/uninstall.sh"
echo "${VERSION}" >"${DIST}/VERSION"

OUT="${REPO_ROOT}/dist/${PKG_NAME}.tar.gz"
tar -czf "${OUT}" -C "${REPO_ROOT}/dist" "${PKG_NAME}"

echo "created ${OUT}"
echo "  amd64: ${DIST}/amd64/bin/"
echo "  arm64: ${DIST}/arm64/bin/"
echo "  release assets: ${ASSETS_DIR}/"
