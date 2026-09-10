#!/usr/bin/env bash
set -euo pipefail
umask 077
install -d -m 0700 /var/backups/deep-seeing/hindsight
memory_backup_tmp=$(mktemp /var/backups/deep-seeing/hindsight/.dump-XXXXXX)
trap 'rm -f -- "$memory_backup_tmp"' EXIT
docker exec deep-seeing-memory-pg pg_dump -U ds_memory_admin -d hindsight -Fc > "$memory_backup_tmp"
docker exec -i deep-seeing-memory-pg pg_restore --list < "$memory_backup_tmp" > /dev/null
memory_backup_stamp=$(date -u +%Y%m%dT%H%M%SZ)
mv -n -- "$memory_backup_tmp" "/var/backups/deep-seeing/hindsight/hindsight-$memory_backup_stamp.dump"
# Only archives produced by this dedicated job; existing infrastructure backups are untouched.
find /var/backups/deep-seeing/hindsight -maxdepth 1 -type f -name 'hindsight-????????T??????Z.dump' -mtime +14 -delete
printf 'Hindsight backup verified: %s\n' "$memory_backup_stamp"
