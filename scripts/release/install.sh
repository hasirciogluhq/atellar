#!/usr/bin/env bash
set -euo pipefail

# Atellar release installer. Downloads and installs the atellar binary and the
# database migrations. It does not start any service.
#
#   curl -fsSL https://github.com/hasirciogluhq/atellar/releases/latest/download/install.sh | sudo bash
#   curl -fsSL .../install.sh | sudo bash -s -- --version v0.1.0
#   sudo ./install.sh --local          # from extracted tarball

# Set at release build time by package.sh (empty in repo = prompt/latest).
RELEASE_VERSION=""

GITHUB_REPO="hasirciogluhq/atellar"
INSTALL_BIN="/usr/local/bin"
INSTALL_SHARE="/usr/share/atellar"
CONFIG_DIR="/etc/atellar"
LOG_DIR="/var/log/atellar"

VERSION=""
LOCAL=0
USE_LATEST=0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR=""
ARCH=""

die() { echo "error: $*" >&2; exit 1; }
require_root() { [[ "${EUID}" -eq 0 ]] || die "run as root (sudo)"; }

usage() {
  cat <<'EOF'
Atellar install.sh

Installs the atellar binary and the database migrations.
Auto-detects linux/amd64 or linux/arm64 from the universal release tarball.
Does not start any service; see the next steps printed after install.

Options:
  --version <tag>   install specific version (e.g. v0.1.0)
  --latest          install latest GitHub release (default when non-interactive)
  --local           use package next to this script (extracted tarball)
  -h, --help        show this help
EOF
  exit 0
}

detect_arch() {
  local machine
  machine="$(uname -m)"
  case "${machine}" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) die "unsupported arch: ${machine} (linux amd64/arm64 only)" ;;
  esac
}

detect_os() {
  local os
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "${os}" in
    linux) ;;
    *) die "unsupported os: ${os} (linux only)" ;;
  esac
}

fetch_latest_version() {
  local tag
  tag="$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
    | grep -m1 '"tag_name"' \
    | sed -E 's/.*"tag_name":[[:space:]]*"v?([^"]+)".*/\1/')"
  [[ -n "${tag}" ]] || die "failed to fetch latest release"
  echo "${tag}"
}

prompt_version() {
  local input=""
  echo ""
  echo "Atellar install"
  echo "GitHub: https://github.com/${GITHUB_REPO}/releases"
  echo ""
  if [[ -t 0 ]]; then
    read -r -p "Version to install (e.g. v0.1.0, empty = latest): " input
  elif [[ -e /dev/tty ]]; then
    read -r -p "Version to install (e.g. v0.1.0, empty = latest): " input < /dev/tty
  else
    USE_LATEST=1
    return
  fi
  if [[ -z "${input}" ]]; then
    USE_LATEST=1
    return
  fi
  VERSION="${input}"
}

resolve_version() {
  if [[ -n "${RELEASE_VERSION}" ]]; then
    VERSION="${RELEASE_VERSION#v}"
    echo "release: v${VERSION}"
    return
  fi
  if [[ "${USE_LATEST}" -eq 1 ]]; then
    VERSION="$(fetch_latest_version)"
    echo "latest release: v${VERSION}"
    return
  fi
  if [[ -z "${VERSION}" ]]; then
    if [[ -t 0 ]] || [[ -e /dev/tty ]]; then
      prompt_version
      [[ "${USE_LATEST}" -eq 1 ]] && VERSION="$(fetch_latest_version)" && echo "latest release: v${VERSION}"
    else
      VERSION="$(fetch_latest_version)"
      echo "latest release: v${VERSION}"
    fi
  fi
  VERSION="${VERSION#v}"
}

find_extracted_root() {
  local base="$1"
  local ver="$2"
  local candidates=(
    "${base}/atellar_${ver}_linux"
    "${base}/atellar-${ver}"
    "${base}"
  )
  for dir in "${candidates[@]}"; do
    if [[ -d "${dir}/migrations" ]]; then
      echo "${dir}"
      return 0
    fi
  done
  die "failed to extract release package (atellar_${ver}_linux not found)"
}

resolve_bin_dir() {
  local root="$1"
  local arch="$2"

  if [[ -d "${root}/${arch}/bin" ]]; then
    echo "${root}/${arch}/bin"
    return
  fi
  if [[ -d "${root}/bin" ]]; then
    echo "${root}/bin"
    return
  fi
  die "binary directory not found: ${root}/${arch}/bin"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --latest) USE_LATEST=1; shift ;;
    --local) LOCAL=1; shift ;;
    -h|--help) usage ;;
    *) die "unknown argument: $1" ;;
  esac
done

require_root
detect_os
ARCH="$(detect_arch)"
echo "platform: linux/${ARCH}"

if [[ "${LOCAL}" -eq 1 ]] || [[ -d "${SCRIPT_DIR}/amd64/bin" ]] || [[ -d "${SCRIPT_DIR}/bin" ]]; then
  WORK_DIR="${SCRIPT_DIR}"
  echo "using local release package: ${WORK_DIR}"
else
  resolve_version
  TARBALL="atellar_${VERSION}_linux.tar.gz"
  URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${TARBALL}"
  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "${TMP_DIR}"' EXIT
  echo "downloading: ${URL}"
  curl -fsSL "${URL}" -o "${TMP_DIR}/${TARBALL}"
  tar -xzf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}"
  WORK_DIR="$(find_extracted_root "${TMP_DIR}" "${VERSION}")"
fi

BIN_DIR="$(resolve_bin_dir "${WORK_DIR}" "${ARCH}")"
echo "binary source: ${BIN_DIR}"

[[ -f "${BIN_DIR}/atellar" ]] || die "binary not found: ${BIN_DIR}/atellar"
[[ -d "${WORK_DIR}/migrations" ]] || die "migrations not found: ${WORK_DIR}/migrations"

echo "installing atellar (linux/${ARCH})..."
install -m 0755 "${BIN_DIR}/atellar" "${INSTALL_BIN}/atellar"

echo "installing migrations..."
mkdir -p "${INSTALL_SHARE}"
rm -rf "${INSTALL_SHARE}/migrations"
cp -a "${WORK_DIR}/migrations" "${INSTALL_SHARE}/migrations"

mkdir -p "${CONFIG_DIR}" "${LOG_DIR}"

INSTALLED_VERSION="${VERSION}"
if [[ -z "${INSTALLED_VERSION}" && -f "${WORK_DIR}/VERSION" ]]; then
  INSTALLED_VERSION="$(tr -d '[:space:]' < "${WORK_DIR}/VERSION" | sed 's/^v//')"
fi
[[ -n "${INSTALLED_VERSION}" ]] || INSTALLED_VERSION="unknown"

cat <<EOF

Atellar v${INSTALLED_VERSION#v} installed (linux/${ARCH}).

Installed files:
  ${INSTALL_BIN}/atellar
  ${INSTALL_SHARE}/migrations
  ${CONFIG_DIR}/          (configuration)
  ${LOG_DIR}/             (container logs)

Next steps:

  # 1) API server (needs PostgreSQL). Prints the generated admin token.
  sudo atellar server install \\
    --database-url "postgresql://user:pass@localhost:5432/atellar?sslmode=disable"

  # 2) Point the CLI at the API server
  atellar config set-cluster default --address <API_HOST> --token <ADMIN_TOKEN>
  atellar config set-context default --cluster default
  atellar config use-context default

  # 3) Create a join token
  atellar join-tokens create

  # 4) On every node (containerd must be running)
  sudo atellar node install
  sudo atellar node join --join-token <TOKEN> --name node-1 \\
    --private-ip <PRIVATE_IP> --control-plane-address <API_HOST>

  # 5) Check the cluster
  atellar nodes list

Documentation: https://github.com/${GITHUB_REPO}/blob/main/docs/getting-started.md

EOF
