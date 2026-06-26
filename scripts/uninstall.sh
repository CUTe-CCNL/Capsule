#!/bin/bash
set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
SERVICE_NAME="${SERVICE_NAME:-host-agent}"
PLUGIN_ID="${PLUGIN_ID:-incus}"
BINARY_NAME="${BINARY_NAME:-incus-plugin}"
PLUGIN_DIR="${PLUGIN_DIR:-/opt/host-agent/plugins}"
INSTALL_DIR="${INSTALL_DIR:-${PLUGIN_DIR}/${PLUGIN_ID}}"
CONFIG_DIR="${CONFIG_DIR:-/etc/host-agent}"
MANIFEST_DIR="${MANIFEST_DIR:-${CONFIG_DIR}/plugins.d}"
MANIFEST_PATH="${MANIFEST_PATH:-${MANIFEST_DIR}/${PLUGIN_ID}.yaml}"
LEGACY_MANIFEST_PATH="${LEGACY_MANIFEST_PATH:-${PLUGIN_DIR}/${PLUGIN_ID}.yaml}"

echo -e "${GREEN}=== Host Agent Incus Plugin uninstaller ===${NC}"

# Check root privileges. Tests can set SKIP_ROOT_CHECK=1.
if [ "${SKIP_ROOT_CHECK:-0}" != "1" ] && [ "$EUID" -ne 0 ]; then
  echo -e "${RED}Please run this script with sudo${NC}"
  exit 1
fi

restart_host_agent() {
  echo -e "${YELLOW}Restarting ${SERVICE_NAME}...${NC}"

  if command -v systemctl >/dev/null 2>&1; then
    systemctl restart "${SERVICE_NAME}"
  elif command -v rc-service >/dev/null 2>&1; then
    rc-service "${SERVICE_NAME}" restart
  else
    echo -e "${YELLOW}Warning: could not find systemctl or rc-service; please restart ${SERVICE_NAME} manually${NC}"
  fi
}

echo "Removing plugin binary..."
rm -f "${INSTALL_DIR}/${BINARY_NAME}"

echo "Removing plugin manifest..."
rm -f "${MANIFEST_PATH}"

if [ "${LEGACY_MANIFEST_PATH}" != "${MANIFEST_PATH}" ] && [ -f "${LEGACY_MANIFEST_PATH}" ]; then
  echo -e "${YELLOW}Removing legacy manifest ${LEGACY_MANIFEST_PATH}...${NC}"
  rm -f "${LEGACY_MANIFEST_PATH}"
fi

rmdir "${INSTALL_DIR}" 2>/dev/null || true

restart_host_agent

echo -e "${GREEN}Uninstall completed.${NC}"
echo "Removed binary from: ${INSTALL_DIR}"
echo "Removed manifest: ${MANIFEST_PATH}"
