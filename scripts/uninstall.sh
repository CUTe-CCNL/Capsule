#!/usr/bin/env bash
set -euo pipefail

plugin_id="${PLUGIN_ID:-incus}"
binary_name="${BINARY_NAME:-incus-plugin}"
plugin_dir="${PLUGIN_DIR:-/opt/host-agent/plugins}"
install_dir="${INSTALL_DIR:-${plugin_dir}/${plugin_id}}"
manifest_path="${MANIFEST_PATH:-${plugin_dir}/${plugin_id}.yaml}"

rm -f "${install_dir}/${binary_name}"
rm -f "${manifest_path}"
rmdir "${install_dir}" 2>/dev/null || true

printf 'Removed %s from %s\n' "${binary_name}" "${install_dir}"
printf 'Removed manifest %s\n' "${manifest_path}"
