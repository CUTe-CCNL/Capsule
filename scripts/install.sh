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

echo -e "${GREEN}=== Host Agent Incus Plugin installer ===${NC}"

# Check root privileges. Tests can set SKIP_ROOT_CHECK=1.
if [ "${SKIP_ROOT_CHECK:-0}" != "1" ] && [ "$EUID" -ne 0 ]; then
  echo -e "${RED}Please run this script with sudo${NC}"
  exit 1
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." 2>/dev/null && pwd)"

bundle_binary="${script_dir}/${BINARY_NAME}"
bundle_manifest="${script_dir}/incus.yaml"
repo_binary="${repo_root}/bin/${BINARY_NAME}"
repo_manifest="${repo_root}/deploy/incus.yaml"

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

if [ -x "${bundle_binary}" ] && [ -f "${bundle_manifest}" ]; then
  source_binary="${bundle_binary}"
  source_manifest="${bundle_manifest}"
elif [ -f "${repo_manifest}" ]; then
  if [ "${SKIP_BUILD:-0}" != "1" ]; then
    echo "Building plugin..."
    (cd "${repo_root}" && make build)
  elif [ ! -x "${repo_binary}" ]; then
    echo "Plugin binary is missing; building plugin..."
    (cd "${repo_root}" && make build)
  fi
  source_binary="${repo_binary}"
  source_manifest="${repo_manifest}"
else
  echo -e "${RED}Could not find ${BINARY_NAME} or deploy/incus.yaml.${NC}" >&2
  echo "Run this script from the repository root or an extracted release bundle." >&2
  exit 1
fi

manifest_tmp="$(mktemp)"
cleanup() {
  rm -f "${manifest_tmp}"
}
trap cleanup EXIT

while IFS= read -r line; do
  case "${line}" in
    command:*)
      printf 'command: "%s/%s"\n' "${INSTALL_DIR}" "${BINARY_NAME}"
      ;;
    working_dir:*)
      printf 'working_dir: "%s"\n' "${INSTALL_DIR}"
      ;;
    *)
      printf '%s\n' "${line}"
      ;;
  esac
done < "${source_manifest}" > "${manifest_tmp}"

echo "Creating directories..."
install -d "${INSTALL_DIR}"
install -d "$(dirname "${MANIFEST_PATH}")"

echo "Installing plugin binary..."
install -m 0755 "${source_binary}" "${INSTALL_DIR}/${BINARY_NAME}"

echo "Installing plugin manifest..."
install -m 0644 "${manifest_tmp}" "${MANIFEST_PATH}"

if [ "${LEGACY_MANIFEST_PATH}" != "${MANIFEST_PATH}" ] && [ -f "${LEGACY_MANIFEST_PATH}" ]; then
  echo -e "${YELLOW}Removing legacy manifest ${LEGACY_MANIFEST_PATH}...${NC}"
  rm -f "${LEGACY_MANIFEST_PATH}"
fi

restart_host_agent

echo -e "${GREEN}Install completed.${NC}"
echo "Binary: ${INSTALL_DIR}/${BINARY_NAME}"
echo "Manifest: ${MANIFEST_PATH}"
