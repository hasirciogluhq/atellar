#!/usr/bin/env bash
set -euo pipefail

# Atellar release installer — downloads and installs binaries only.
# Does not start services or run atelctl.
#
#   curl -fsSL https://github.com/hasirciogluhq/atellar/releases/latest/download/install.sh | sudo bash
#   sudo ./install.sh --local          # from extracted tarball

GITHUB_REPO="hasirciogluhq/atellar"
INSTALL_BIN="/usr/local/bin"
INSTALL_SHARE="/usr/share/atellar"
CONFIG_DIR="/etc/atellar"
LOG_DIR="/var/log/atellar"

VERSION=""
LOCAL=0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR=""

die() { echo "error: $*" >&2; exit 1; }
require_root() { [[ "${EUID}" -eq 0 ]] || die "run as root (sudo)"; }

usage() {
  cat <<'EOF'
Atellar install.sh

Installs atellar-api, atellar-agent, atelctl and DB migrations.
Does not start any service — configure and run manually after install.

Options:
  --version <tag>   skip prompt (e.g. v0.1.0)
  --local           use ./bin and ./migrations next to this script
  -h, --help        show this help
EOF
  exit 0
}

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
    *) die "unsupported os: ${os} (linux only)" ;;
  esac
  echo "${os}_${arch}"
}

prompt_version() {
  local input=""
  echo ""
  echo "Atellar kurulumu"
  echo "GitHub: https://github.com/${GITHUB_REPO}/releases"
  echo ""
  if [[ -t 0 ]]; then
    read -r -p "Kurulacak versiyon (örn. v0.1.0): " input
  elif [[ -e /dev/tty ]]; then
    read -r -p "Kurulacak versiyon (örn. v0.1.0): " input < /dev/tty
  else
    die "versiyon gerekli (etkileşimsiz ortam: --version v0.1.0 kullanın)"
  fi
  [[ -n "${input}" ]] || die "versiyon boş olamaz"
  VERSION="${input}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --local) LOCAL=1; shift ;;
    -h|--help) usage ;;
    *) die "bilinmeyen argüman: $1" ;;
  esac
done

require_root
platform="$(detect_platform)"

if [[ "${LOCAL}" -eq 1 ]] || [[ -f "${SCRIPT_DIR}/bin/atellar-api" ]]; then
  WORK_DIR="${SCRIPT_DIR}"
  echo "yerel release paketi kullanılıyor: ${WORK_DIR}"
else
  [[ -z "${VERSION}" ]] && prompt_version
  VERSION="${VERSION#v}"
  TARBALL="atellar_${VERSION}_${platform}.tar.gz"
  URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${TARBALL}"
  WORK_DIR="$(mktemp -d)"
  trap 'rm -rf "${WORK_DIR}"' EXIT
  echo "indiriliyor: ${URL}"
  curl -fsSL "${URL}" -o "${WORK_DIR}/${TARBALL}"
  tar -xzf "${WORK_DIR}/${TARBALL}" -C "${WORK_DIR}"
  if [[ -d "${WORK_DIR}/atellar_${VERSION}_${platform}" ]]; then
    WORK_DIR="${WORK_DIR}/atellar_${VERSION}_${platform}"
  elif [[ -d "${WORK_DIR}/atellar-${VERSION}" ]]; then
    WORK_DIR="${WORK_DIR}/atellar-${VERSION}"
  fi
fi

for bin in atellar-api atellar-agent atelctl; do
  [[ -f "${WORK_DIR}/bin/${bin}" ]] || die "binary bulunamadı: ${WORK_DIR}/bin/${bin}"
done
[[ -d "${WORK_DIR}/migrations" ]] || die "migrations bulunamadı: ${WORK_DIR}/migrations"

echo "binary'ler kuruluyor..."
install -m 0755 "${WORK_DIR}/bin/atellar-api" "${INSTALL_BIN}/atellar-api"
install -m 0755 "${WORK_DIR}/bin/atellar-agent" "${INSTALL_BIN}/atellar-agent"
install -m 0755 "${WORK_DIR}/bin/atelctl" "${INSTALL_BIN}/atelctl"

echo "migrations kuruluyor..."
mkdir -p "${INSTALL_SHARE}"
rm -rf "${INSTALL_SHARE}/migrations"
cp -a "${WORK_DIR}/migrations" "${INSTALL_SHARE}/migrations"

mkdir -p "${CONFIG_DIR}" "${LOG_DIR}"

INSTALLED_VERSION="${VERSION}"
if [[ -z "${INSTALLED_VERSION}" && -f "${WORK_DIR}/VERSION" ]]; then
  INSTALLED_VERSION="$(tr -d '[:space:]' < "${WORK_DIR}/VERSION")"
fi
[[ -n "${INSTALLED_VERSION}" ]] || INSTALLED_VERSION="unknown"

cat <<EOF

Merhaba — Atellar v${INSTALLED_VERSION#v} kuruldu.

Kurulan dosyalar:
  ${INSTALL_BIN}/atellar-api
  ${INSTALL_BIN}/atellar-agent
  ${INSTALL_BIN}/atelctl
  ${INSTALL_SHARE}/migrations
  ${CONFIG_DIR}/          (config dizini)
  ${LOG_DIR}/               (log dizini)

Bundan sonra örnek adımlar:

  # 1) Control plane (PostgreSQL gerekli)
  export DATABASE_URL="postgresql://user:pass@localhost:5432/atellar_cp?sslmode=disable"
  export MIGRATIONS_PATH="${INSTALL_SHARE}/migrations"
  export PORT=8080
  export GRPC_PORT=9090
  atellar-api

  # 2) Join token oluştur
  curl -X POST http://localhost:8080/api/v1/nodes/join-tokens \\
    -H "Content-Type: application/json" \\
    -d '{"single_use": true}'

  # 3) Agent kur + cluster'a katıl
  atelctl agent install --auto-join \\
    --join-token <TOKEN> \\
    --name node-1 \\
    --public-ip <PUBLIC_IP> \\
    --private-ip <PRIVATE_IP> \\
    --control-plane-address <CP_HOST> \\
    --http-port 8080 \\
    --grpc-port 9090

  # 4) Cluster durumu
  atelctl cluster nodes list \\
    --control-plane-address <CP_HOST> --http-port 8080 --grpc-port 9090

Dokümantasyon: https://github.com/${GITHUB_REPO}/blob/main/docs/getting-started.md

EOF
