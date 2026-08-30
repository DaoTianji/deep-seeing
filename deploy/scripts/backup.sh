#!/usr/bin/env bash
set -euo pipefail

umask 077

readonly service_name="deep-seeing.service"
readonly state_root="/var/lib/deep-seeing"
readonly seed_parent="/opt/deep-seeing"
readonly backup_root="/var/backups/deep-seeing"
readonly stamp="$(date -u +%Y%m%dT%H%M%SZ)"
readonly pending="${backup_root}/.deep-seeing-${stamp}.tar.gz"
readonly archive="${backup_root}/deep-seeing-${stamp}.tar.gz"

install -d -m 0700 "${backup_root}"

was_active=0
if systemctl is-active --quiet "${service_name}"; then
  was_active=1
  systemctl stop "${service_name}"
fi

restore_service() {
  rm -f -- "${pending}"
  if [[ "${was_active}" -eq 1 ]]; then
    systemctl start "${service_name}"
  fi
}
trap restore_service EXIT

tar \
  --create \
  --gzip \
  --file "${pending}" \
  --numeric-owner \
  --directory "${state_root}" data \
  --directory "${seed_parent}" seed

mv -- "${pending}" "${archive}"
sha256sum "${archive}" > "${archive}.sha256"

if [[ "${was_active}" -eq 1 ]]; then
  systemctl start "${service_name}"
  was_active=0
fi
trap - EXIT

find "${backup_root}" -maxdepth 1 -type f \
  \( -name 'deep-seeing-*.tar.gz' -o -name 'deep-seeing-*.tar.gz.sha256' \) \
  -mtime +14 -delete

printf '%s\n' "${archive}"
