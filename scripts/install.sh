#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." 2>/dev/null && pwd)"

plugin_id="${PLUGIN_ID:-incus}"
binary_name="${BINARY_NAME:-incus-plugin}"
plugin_dir="${PLUGIN_DIR:-/opt/host-agent/plugins}"
install_dir="${INSTALL_DIR:-${plugin_dir}/${plugin_id}}"
manifest_path="${MANIFEST_PATH:-${plugin_dir}/${plugin_id}.yaml}"

bundle_binary="${script_dir}/${binary_name}"
bundle_manifest="${script_dir}/incus.yaml"
repo_binary="${repo_root}/bin/${binary_name}"
repo_manifest="${repo_root}/deploy/incus.yaml"

if [[ -x "${bundle_binary}" && -f "${bundle_manifest}" ]]; then
  source_binary="${bundle_binary}"
  source_manifest="${bundle_manifest}"
elif [[ -f "${repo_manifest}" ]]; then
  if [[ "${SKIP_BUILD:-0}" != "1" ]]; then
    (cd "${repo_root}" && make build)
  elif [[ ! -x "${repo_binary}" ]]; then
    (cd "${repo_root}" && make build)
  fi
  source_binary="${repo_binary}"
  source_manifest="${repo_manifest}"
else
  printf 'Could not find %s or deploy/incus.yaml. Run from repo root or an extracted release bundle.\n' "${binary_name}" >&2
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
      printf 'command: "%s/%s"\n' "${install_dir}" "${binary_name}"
      ;;
    working_dir:*)
      printf 'working_dir: "%s"\n' "${install_dir}"
      ;;
    *)
      printf '%s\n' "${line}"
      ;;
  esac
done < "${source_manifest}" > "${manifest_tmp}"

install -d "${install_dir}"
install -d "$(dirname "${manifest_path}")"
install -m 0755 "${source_binary}" "${install_dir}/${binary_name}"
install -m 0644 "${manifest_tmp}" "${manifest_path}"

printf 'Installed %s to %s\n' "${binary_name}" "${install_dir}/${binary_name}"
printf 'Installed manifest to %s\n' "${manifest_path}"
