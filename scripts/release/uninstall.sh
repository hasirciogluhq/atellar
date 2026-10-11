#!/usr/bin/env bash
set -euo pipefail

# Set at release build time by package.sh (empty in repo).
RELEASE_VERSION=""

# Removes the atellar binary, configuration, logs, migrations, systemd units,
# local containers and overlay links. It does not remove the node from the API
# server; run `atellar nodes evict <NODE>` for that.

INSTALL_BIN="/usr/local/bin"
INSTALL_SHARE="/usr/share/atellar"
CONFIG_DIR="/etc/atellar"
LOG_DIR="/var/log/atellar"
API_UNIT="/etc/systemd/system/atellar-server.service"
AGENT_UNIT="/etc/systemd/system/atellar-agent.service"
LINKS=(atellar-vx atellar0)
CONTAINERD_NS="atellar"

YES=0

die() { echo "error: $*" >&2; exit 1; }
require_root() { [[ "${EUID}" -eq 0 ]] || die "run as root (sudo)"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --yes|-y) YES=1; shift ;;
    -h|--help)
      echo "usage: sudo ./uninstall.sh [--yes]"
      exit 0
      ;;
    *) die "unknown argument: $1" ;;
  esac
done

require_root

if [[ "${YES}" -ne 1 ]]; then
  echo "this will remove atellar, its configuration, logs, migrations and local containers."
  read -r -p "continue? [y/N] " reply
  [[ "${reply}" =~ ^[Yy]$ ]] || die "aborted"
fi

for svc in atellar-agent atellar-server; do
  systemctl stop "${svc}" 2>/dev/null || true
  systemctl disable "${svc}" 2>/dev/null || true
done

rm -f "${API_UNIT}" "${AGENT_UNIT}"
systemctl daemon-reload

if command -v ctr >/dev/null 2>&1; then
  if ctr namespace ls 2>/dev/null | awk '{print $1}' | grep -qx "${CONTAINERD_NS}"; then
    mapfile -t containers < <(ctr -n "${CONTAINERD_NS}" containers ls -q 2>/dev/null || true)
    for id in "${containers[@]:-}"; do
      [[ -z "${id}" ]] && continue
      ctr -n "${CONTAINERD_NS}" tasks kill -a "${id}" 2>/dev/null || true
      ctr -n "${CONTAINERD_NS}" tasks rm "${id}" 2>/dev/null || true
      ctr -n "${CONTAINERD_NS}" containers rm "${id}" 2>/dev/null || true
    done
  fi
fi

if command -v ip >/dev/null 2>&1; then
  for link in "${LINKS[@]}"; do
    ip link show "${link}" >/dev/null 2>&1 || continue
    ip link set "${link}" down 2>/dev/null || true
    ip link del "${link}" 2>/dev/null || true
  done
fi

rm -rf "${CONFIG_DIR}" "${LOG_DIR}" "${INSTALL_SHARE}"
rm -f "${INSTALL_BIN}/atellar"

if [[ -n "${RELEASE_VERSION}" ]]; then
  echo "Atellar v${RELEASE_VERSION#v} uninstalled."
else
  echo "uninstall complete."
fi
