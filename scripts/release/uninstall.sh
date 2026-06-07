#!/usr/bin/env bash
set -euo pipefail

# Set at release build time by package.sh (empty in repo).
RELEASE_VERSION=""

# Removes atellar-api, atellar-agent, atelctl, config, logs, migrations, systemd units.
# Does NOT remove the node from control plane DB — evict via API separately.

INSTALL_BIN="/usr/local/bin"
INSTALL_SHARE="/usr/share/atellar"
CONFIG_DIR="/etc/atellar"
LOG_DIR="/var/log/atellar"
API_UNIT="/etc/systemd/system/atellar-api.service"
AGENT_UNIT="/etc/systemd/system/atellar-agent.service"
BRIDGE_NAME="atellar0"
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
  echo "this will remove atellar-api, atellar-agent, atelctl, config, logs, migrations, and local workloads."
  read -r -p "continue? [y/N] " reply
  [[ "${reply}" =~ ^[Yy]$ ]] || die "aborted"
fi

for svc in atellar-agent atellar-api; do
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

if command -v ip >/dev/null 2>&1 && ip link show "${BRIDGE_NAME}" >/dev/null 2>&1; then
  ip link set "${BRIDGE_NAME}" down 2>/dev/null || true
  ip link del "${BRIDGE_NAME}" 2>/dev/null || true
fi

rm -rf "${CONFIG_DIR}" "${LOG_DIR}" "${INSTALL_SHARE}"
rm -f "${INSTALL_BIN}/atellar-api" "${INSTALL_BIN}/atellar-agent" "${INSTALL_BIN}/atelctl"

if [[ -n "${RELEASE_VERSION}" ]]; then
  echo "Atellar v${RELEASE_VERSION#v} uninstalled."
else
  echo "uninstall complete."
fi
