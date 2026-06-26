#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

binary_name="${BINARY_NAME:-incus-plugin}"
cmd_dir="${CMD_DIR:-./cmd/incus-plugin}"
dist_dir="${DIST_DIR:-${repo_root}/dist}"
goos="${GOOS:-$(go env GOOS)}"
goarch="${GOARCH:-$(go env GOARCH)}"
version="${VERSION:-$(git -C "${repo_root}" describe --tags --always --dirty 2>/dev/null || printf 'dev')}"
build_time="$(date -u '+%Y-%m-%d_%H:%M:%S')"
archive_name="${binary_name}-${version}-${goos}-${goarch}.tar.gz"
archive_path="${dist_dir}/${archive_name}"

if [[ "${SKIP_TESTS:-0}" != "1" ]]; then
  (cd "${repo_root}" && go test ./...)
fi

mkdir -p "${dist_dir}" "${repo_root}/bin"

(
  cd "${repo_root}"
  GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED="${CGO_ENABLED:-0}" \
    go build \
      -buildvcs=false \
      -ldflags "-X main.version=${version} -X main.buildTime=${build_time} -s -w" \
      -o "${repo_root}/bin/${binary_name}" \
      "${cmd_dir}"
)

staging_dir="$(mktemp -d)"
cleanup() {
  rm -rf "${staging_dir}"
}
trap cleanup EXIT

install -m 0755 "${repo_root}/bin/${binary_name}" "${staging_dir}/${binary_name}"
install -m 0644 "${repo_root}/deploy/incus.yaml" "${staging_dir}/incus.yaml"
install -m 0755 "${repo_root}/scripts/install.sh" "${staging_dir}/install.sh"
install -m 0755 "${repo_root}/scripts/uninstall.sh" "${staging_dir}/uninstall.sh"
install -m 0644 "${repo_root}/README.md" "${staging_dir}/README.md"
install -m 0644 "${repo_root}/LICENSE" "${staging_dir}/LICENSE"

tar -C "${staging_dir}" -czf "${archive_path}" \
  README.md \
  LICENSE \
  "${binary_name}" \
  incus.yaml \
  install.sh \
  uninstall.sh

printf 'Created %s\n' "${archive_path}"
