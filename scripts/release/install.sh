#!/usr/bin/env bash
set -euo pipefail

# Atellar release installer — installs control plane, agent, and atelctl binaries.
#
# Run from extracted release tarball:
#   sudo ./install.sh [options]
#
# Or download a release and install:
#   curl -fsSL https://github.com/hasirciogluhq/atellar/releases/latest/download/install.sh | sudo bash -s -- --version v0.1.0 ...
#
# Installs to:
#   /usr/local/bin/atellar-api
#   /usr/local/bin/atellar-agent
#   /usr/local/bin/atelctl
#   /usr/share/atellar/migrations
#
# Control plane (optional):
#   --database-url <postgres-url>
#   --http-port 8080 --grpc-port 9090
#
# Agent join (optional):
#   --join-token --name --public-ip --private-ip
#   --control-plane-address --http-port --grpc-port  (CP address seen by this node)

GITHUB_REPO="hasirciogluhq/atellar"
INSTALL_BIN="/usr/local/bin"
INSTALL_SHARE="/usr/share/atellar"
CONFIG_DIR="/etc/atellar"
LOG_DIR="/var/log/atellar"
API_ENV="${CONFIG_DIR}/api.env"
API_UNIT="/etc/systemd/system/atellar-api.service"
AGENT_UNIT="/etc/systemd/system/atellar-agent.service"

VERSION=""
DOWNLOAD=1
DATABASE_URL=""
HTTP_PORT="8080"
GRPC_PORT="9090"

JOIN_TOKEN=""
NODE_NAME=""
PUBLIC_IP=""
PRIVATE_IP=""
CP_ADDRESS=""
SETUP_AGENT_JOIN=0
CONTAINERD_SOCK="/run/containerd/containerd.sock"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR=""

usage() {
  cat <<'EOF'
Atellar install.sh

Installs: atellar-api, atellar-agent, atelctl

Options:
  --version <tag>              release tag (e.g. v0.1.0); default: use ./bin next to this script
  --local                      do not download; use ./bin and ./migrations in bundle
  --database-url <url>         configure and enable control plane (atellar-api)
  --http-port <port>           HTTP port (default 8080)
  --grpc-port <port>           gRPC port (default 9090)

Agent join (optional, runs atelctl agent install --auto-join):
  --join-token <token>
  --name <node-name>
  --public-ip <ip>
  --private-ip <ip>
  --control-plane-address <host>
  --containerd-sock <path>     default /run/containerd/containerd.sock

Examples:
  sudo ./install.sh --local --database-url 'postgres://...'

  sudo ./install.sh --version v0.1.0 \
    --join-token TOKEN --name node-1 \
    --public-ip 1.2.3.4 --private-ip 10.0.0.5 \
    --control-plane-address 10.0.0.1 --http-port 8080 --grpc-port 9090
EOF
  exit 1
}

die() { echo "error: $*" >&2; exit 1; }
require_root() { [[ "${EUID}" -eq 0 ]] || die "run as root (sudo)"; }

detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "${arch}" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) die "unsupported arch: ${arch}" ;;
  esac
  case "${os}" in
    linux) ;;
    darwin) die "macOS binaries are not supported by this installer yet" ;;
    *) die "unsupported os: ${os}" ;;
  esac
  echo "${os}_${arch}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --local) DOWNLOAD=0; shift ;;
    --database-url) DATABASE_URL="${2:-}"; shift 2 ;;
    --http-port) HTTP_PORT="${2:-}"; shift 2 ;;
    --grpc-port) GRPC_PORT="${2:-}"; shift 2 ;;
    --join-token) JOIN_TOKEN="${2:-}"; SETUP_AGENT_JOIN=1; shift 2 ;;
    --name) NODE_NAME="${2:-}"; SETUP_AGENT_JOIN=1; shift 2 ;;
    --public-ip) PUBLIC_IP="${2:-}"; SETUP_AGENT_JOIN=1; shift 2 ;;
    --private-ip) PRIVATE_IP="${2:-}"; SETUP_AGENT_JOIN=1; shift 2 ;;
    --control-plane-address) CP_ADDRESS="${2:-}"; SETUP_AGENT_JOIN=1; shift 2 ;;
    --containerd-sock) CONTAINERD_SOCK="${2:-}"; shift 2 ;;
    -h|--help) usage ;;
    *) die "unknown argument: $1" ;;
  esac
done

require_root

if [[ "${SETUP_AGENT_JOIN}" -eq 1 ]]; then
  [[ -n "${JOIN_TOKEN}" && -n "${NODE_NAME}" && -n "${PUBLIC_IP}" && -n "${PRIVATE_IP}" && -n "${CP_ADDRESS}" ]] \
    || die "agent join requires --join-token --name --public-ip --private-ip --control-plane-address"
  [[ -n "${HTTP_PORT}" && -n "${GRPC_PORT}" ]] || die "--http-port and --grpc-port are required for agent join"
  [[ -S "${CONTAINERD_SOCK}" ]] || die "containerd socket not found: ${CONTAINERD_SOCK}"
fi

platform="$(detect_platform)"

if [[ "${DOWNLOAD}" -eq 0 ]] || [[ -f "${SCRIPT_DIR}/bin/atellar-api" ]]; then
  WORK_DIR="${SCRIPT_DIR}"
  echo "using local release bundle: ${WORK_DIR}"
else
  [[ -n "${VERSION}" ]] || die "--version is required when not running from a release bundle (or use --local)"
  VERSION="${VERSION#v}"
  TARBALL="atellar_${VERSION}_${platform}.tar.gz"
  URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${TARBALL}"
  WORK_DIR="$(mktemp -d)"
  trap 'rm -rf "${WORK_DIR}"' EXIT
  echo "downloading ${URL}..."
  curl -fsSL "${URL}" -o "${WORK_DIR}/${TARBALL}"
  tar -xzf "${WORK_DIR}/${TARBALL}" -C "${WORK_DIR}"
  # tarball root may be atellar_vX.Y.Z_linux_amd64/ or flat
  if [[ -d "${WORK_DIR}/atellar_${VERSION}_${platform}" ]]; then
    WORK_DIR="${WORK_DIR}/atellar_${VERSION}_${platform}"
  elif [[ -d "${WORK_DIR}/atellar-${VERSION}" ]]; then
    WORK_DIR="${WORK_DIR}/atellar-${VERSION}"
  fi
fi

for bin in atellar-api atellar-agent atelctl; do
  [[ -f "${WORK_DIR}/bin/${bin}" ]] || die "missing binary: ${WORK_DIR}/bin/${bin}"
done

[[ -d "${WORK_DIR}/migrations" ]] || die "missing migrations: ${WORK_DIR}/migrations"

echo "installing binaries..."
install -m 0755 "${WORK_DIR}/bin/atellar-api" "${INSTALL_BIN}/atellar-api"
install -m 0755 "${WORK_DIR}/bin/atellar-agent" "${INSTALL_BIN}/atellar-agent"
install -m 0755 "${WORK_DIR}/bin/atelctl" "${INSTALL_BIN}/atelctl"

echo "installing migrations..."
mkdir -p "${INSTALL_SHARE}"
rm -rf "${INSTALL_SHARE}/migrations"
cp -a "${WORK_DIR}/migrations" "${INSTALL_SHARE}/migrations"

mkdir -p "${CONFIG_DIR}" "${LOG_DIR}"

cat >"${API_UNIT}" <<EOF
[Unit]
Description=Atellar Control Plane API
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=${API_ENV}
ExecStart=${INSTALL_BIN}/atellar-api
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

cat >"${AGENT_UNIT}" <<EOF
[Unit]
Description=Atellar Node Agent
After=network-online.target containerd.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_BIN}/atellar-agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable atellar-api.service atellar-agent.service

if [[ -n "${DATABASE_URL}" ]]; then
  echo "configuring control plane..."
  cat >"${API_ENV}" <<EOF
DATABASE_URL=${DATABASE_URL}
MIGRATIONS_PATH=${INSTALL_SHARE}/migrations
PORT=${HTTP_PORT}
GRPC_PORT=${GRPC_PORT}
EOF
  chmod 0600 "${API_ENV}"
  systemctl restart atellar-api.service
  echo "control plane started (atellar-api)"
else
  echo "skip control plane start (pass --database-url to configure atellar-api)"
fi

if [[ "${SETUP_AGENT_JOIN}" -eq 1 ]]; then
  echo "joining node to cluster..."
  atelctl agent install --auto-join \
    --join-token "${JOIN_TOKEN}" \
    --name "${NODE_NAME}" \
    --public-ip "${PUBLIC_IP}" \
    --private-ip "${PRIVATE_IP}" \
    --control-plane-address "${CP_ADDRESS}" \
    --http-port "${HTTP_PORT}" \
    --grpc-port "${GRPC_PORT}" \
    --containerd-sock "${CONTAINERD_SOCK}"
else
  echo "skip agent join (pass --join-token ... to register this node)"
  systemctl stop atellar-agent.service 2>/dev/null || true
fi

echo ""
echo "installed:"
echo "  ${INSTALL_BIN}/atellar-api"
echo "  ${INSTALL_BIN}/atellar-agent"
echo "  ${INSTALL_BIN}/atelctl"
echo "  ${INSTALL_SHARE}/migrations"
echo ""
echo "status:"
echo "  systemctl status atellar-api"
echo "  systemctl status atellar-agent"
