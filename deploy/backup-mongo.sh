#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
compose_file="${QUIZBATTLE_COMPOSE_FILE:-${script_dir}/docker-compose.production.yml}"
env_file="${QUIZBATTLE_ENV_FILE:-${script_dir}/.env}"
secret_dir="${QUIZBATTLE_SECRET_DIR:-${script_dir}/secrets}"
backup_dir="/var/backups/quizbattle/mongo"
recipient_certificate="${secret_dir}/backup-recipient.pem"
lock_file="${backup_dir}/.backup.lock"
retention_days="${QUIZBATTLE_BACKUP_RETENTION_DAYS:-14}"

fail() {
  printf 'backup error: %s\n' "$*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "run backups as root"
for command_name in docker openssl sha256sum flock install stat readlink find awk grep; do
  command -v "${command_name}" >/dev/null 2>&1 || fail "required command is missing: ${command_name}"
done
docker compose version >/dev/null 2>&1 || fail "Docker Compose v2 is required"

[[ -f "${compose_file}" && ! -L "${compose_file}" ]] || fail "production Compose file is missing"
[[ -f "${env_file}" && ! -L "${env_file}" ]] || fail "root-only .env file is missing"
[[ "$(stat -c '%u:%g:%a' "${env_file}")" == "0:0:600" ]] || fail ".env must be root:root mode 0600"
[[ -f "${recipient_certificate}" && ! -L "${recipient_certificate}" ]] || fail "backup recipient certificate is missing"
[[ "$(stat -c '%u:%g:%a' "${recipient_certificate}")" == "0:0:644" ]] || fail "backup recipient certificate must be root:root mode 0644"
openssl x509 -checkend 86400 -noout -in "${recipient_certificate}" >/dev/null || fail "backup recipient certificate expires within 24 hours"
[[ "${retention_days}" =~ ^[0-9]{1,4}$ ]] || fail "backup retention must be between 1 and 3650 days"
((retention_days >= 1 && retention_days <= 3650)) || fail "backup retention must be between 1 and 3650 days"

if [[ -e "${backup_dir}" || -L "${backup_dir}" ]]; then
  [[ -d "${backup_dir}" && ! -L "${backup_dir}" ]] || fail "backup destination must be a real directory"
fi
install -d -o root -g root -m 0700 "${backup_dir}"
[[ "$(readlink -f "${backup_dir}")" == "${backup_dir}" ]] || fail "backup path must not traverse symlinks"
[[ "$(stat -c '%u:%g:%a' "${backup_dir}")" == "0:0:700" ]] || fail "backup directory must be root:root mode 0700"

if [[ -e "${lock_file}" || -L "${lock_file}" ]]; then
  [[ -f "${lock_file}" && ! -L "${lock_file}" ]] || fail "backup lock must be a regular file"
else
  (set -o noclobber; umask 077; : >"${lock_file}") \
    || fail "backup lock could not be created safely"
fi
[[ "$(stat -c '%u:%g:%a:%h' "${lock_file}")" == "0:0:600:1" ]] \
  || fail "backup lock must be root:root mode 0600 with one link"
exec 9<>"${lock_file}"
flock -n 9 || fail "another QuizBattle Mongo backup is already running"

compose=(docker compose --project-name quizbattle --env-file "${env_file}" -f "${compose_file}")
"${compose[@]}" config --quiet
"${compose[@]}" ps --status running --services | grep -qx 'mongo' || fail "Mongo service is not running"

timestamp="$(date -u +'%Y%m%dT%H%M%SZ')"
filename="quizbattle-mongo-${timestamp}.archive.gz.p7m"
destination="${backup_dir}/${filename}"
partial="${destination}.partial"
checksum="${destination}.sha256"
checksum_partial="${checksum}.partial"

cleanup() {
  rm -f -- "${partial}" "${checksum_partial}"
}
trap cleanup EXIT

# The variables in this script fragment are intentionally evaluated in Mongo.
# shellcheck disable=SC2016
"${compose[@]}" exec -T mongo bash -euc '
  root_password="$(< /run/quizbattle-mongo/root-password)"
  config_file="$(mktemp /tmp/quizbattle-mongodump.XXXXXX.yml)"
  cleanup_config() {
    [[ -z "${config_file:-}" ]] || rm -f -- "${config_file}"
    unset root_password config_file
  }
  trap cleanup_config EXIT HUP INT TERM
  printf "password: %s\n" "${root_password}" >"${config_file}"
  chmod 0600 "${config_file}"
  unset root_password
  mongodump \
    --quiet \
    --config "${config_file}" \
    --host mongo:27017 \
    --username "${MONGO_INITDB_ROOT_USERNAME}" \
    --authenticationDatabase admin \
    --authenticationMechanism SCRAM-SHA-256 \
    --ssl \
    --sslCAFile /run/quizbattle-mongo/ca.crt \
    --readPreference primary \
    --oplog \
    --archive \
    --gzip
' | openssl cms -encrypt -binary -stream -aes-256-gcm -outform DER -out "${partial}" "${recipient_certificate}"

[[ -s "${partial}" ]] || fail "encrypted backup is empty"
openssl cms -cmsout -inform DER -in "${partial}" -noout >/dev/null || fail "encrypted backup envelope failed validation"
digest="$(sha256sum "${partial}" | awk '{print $1}')"
printf '%s  %s\n' "${digest}" "${filename}" >"${checksum_partial}"
chmod 0600 "${partial}" "${checksum_partial}"
mv -- "${partial}" "${destination}"
mv -- "${checksum_partial}" "${checksum}"
trap - EXIT

# Retention runs only after a complete encrypted backup and is constrained to
# this fixed directory and our timestamped backup/checksum filename pattern.
find "${backup_dir}" -maxdepth 1 -type f \
  \( -name 'quizbattle-mongo-*.archive.gz.p7m' -o -name 'quizbattle-mongo-*.archive.gz.p7m.sha256' \) \
  -mtime "+${retention_days}" -delete

printf 'Encrypted Mongo backup completed: %s\n' "${destination}"
printf 'Encrypted backups older than %s days were pruned from the fixed backup directory.\n' "${retention_days}"
printf 'Keep an offline copy of secrets/backup-private-key.pem to make restoration possible.\n'

# Restore outline (run deliberately, never from this backup path automatically):
# openssl cms -decrypt -binary -inform DER -in BACKUP.p7m \
#   -recip secrets/backup-recipient.pem -inkey secrets/backup-private-key.pem |
# docker compose --project-name quizbattle --env-file .env \
#   -f docker-compose.production.yml exec -T mongo bash -euc '
#     root_password="$(< /run/quizbattle-mongo/root-password)"
#     config_file="$(mktemp /tmp/quizbattle-mongorestore.XXXXXX.yml)"
#     trap "rm -f -- ${config_file}; unset root_password config_file" EXIT
#     printf "password: %s\n" "${root_password}" >"${config_file}"
#     chmod 0600 "${config_file}"
#     unset root_password
#     mongorestore --host mongo:27017 \
#       --config "${config_file}" --username "${MONGO_INITDB_ROOT_USERNAME}" \
#       --authenticationDatabase admin --authenticationMechanism SCRAM-SHA-256 \
#       --ssl --sslCAFile /run/quizbattle-mongo/ca.crt \
#       --archive --gzip --oplogReplay
#   '
