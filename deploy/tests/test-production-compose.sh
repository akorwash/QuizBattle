#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

fail() {
  printf 'production Compose integration failed: %s\n' "$*" >&2
  exit 1
}

[[ "${GITHUB_ACTIONS:-}" == "true" ]] \
  || fail "this destructive fresh-volume integration is restricted to an ephemeral GitHub Actions runner"
[[ "${EUID}" -eq 0 ]] || fail "run the integration through sudo"

app_image="${1:-}"
release_sha="${2:-}"
[[ "$app_image" =~ ^[A-Za-z0-9._:/-]+$ ]] || fail "a local CI image tag is required"
[[ "$release_sha" =~ ^[0-9a-f]{40}$ ]] || fail "a full lowercase release SHA is required"

for command_name in awk cmp cp curl dd docker find gzip install mktemp od openssl readlink sha256sum stat; do
  command -v "$command_name" >/dev/null 2>&1 || fail "required command is missing: ${command_name}"
done
docker compose version >/dev/null 2>&1 || fail "Docker Compose v2 is required"
docker image inspect "$app_image" >/dev/null 2>&1 || fail "the locally built application image is unavailable"
[[ ! -e /var/backups/quizbattle && ! -L /var/backups/quizbattle ]] \
  || fail "the ephemeral runner unexpectedly contains a QuizBattle backup path"

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd -P)"
fixture_dir="$(mktemp -d /tmp/quizbattle-production-integration.XXXXXX)"
compose_file="$fixture_dir/docker-compose.production.yml"
env_file="$fixture_dir/.env"
secret_dir="$fixture_dir/secrets"
backup_file=""

install -m 0644 "$repo_root/deploy/docker-compose.production.yml" "$compose_file"
install -m 0750 "$repo_root/deploy/bootstrap-secrets.sh" "$fixture_dir/bootstrap-secrets.sh"
install -m 0750 "$repo_root/deploy/mongo-init.sh" "$fixture_dir/mongo-init.sh"
install -m 0750 "$repo_root/deploy/backup-mongo.sh" "$fixture_dir/backup-mongo.sh"

compose=(docker compose --project-name quizbattle --env-file "$env_file" -f "$compose_file")
cleanup() {
  "${compose[@]}" down --volumes --remove-orphans --timeout 30 >/dev/null 2>&1 || true
  if [[ -n "$backup_file" ]]; then
    rm -f -- "$backup_file" "${backup_file}.sha256"
  fi
  rm -f -- /var/backups/quizbattle/mongo/.backup.lock
  rmdir -- /var/backups/quizbattle/mongo /var/backups/quizbattle 2>/dev/null || true
  resolved_fixture="$(readlink -f -- "$fixture_dir" 2>/dev/null || true)"
  case "$resolved_fixture" in
    /tmp/quizbattle-production-integration.*)
      [[ "$resolved_fixture" == "$fixture_dir" ]] \
        && rm -rf -- "$resolved_fixture"
      ;;
    *)
      printf 'refusing to remove unexpected fixture path: %s\n' "$fixture_dir" >&2
      ;;
  esac
}
trap cleanup EXIT

"$fixture_dir/bootstrap-secrets.sh" \
  --bootstrap \
  --origin https://quizbattle.qubefyn.com \
  --env-file "$env_file" \
  --secret-dir "$secret_dir"

# Production requires an immutable GHCR digest, while this pre-publish gate must
# exercise the image built locally by the quality job. Replace only the two release
# identity fields in the generated CI fixture; the production deploy path retains
# its independent repository/digest and OCI-label validation.
env_temporary="$(mktemp "$fixture_dir/.env.integration.XXXXXX")"
awk -v image="$app_image" -v release="$release_sha" '
  /^QUIZBATTLE_IMAGE=/ { print "QUIZBATTLE_IMAGE=" image; next }
  /^RELEASE_SHA=/ { print "RELEASE_SHA=" release; next }
  { print }
' "$env_file" > "$env_temporary"
install -o root -g root -m 0600 "$env_temporary" "$env_file"
rm -f -- "$env_temporary"

"${compose[@]}" config --quiet
"${compose[@]}" up -d --wait --wait-timeout 300

health="$(curl --fail --silent --show-error --max-time 10 http://127.0.0.1:3200/healthz)"
ready="$(curl --fail --silent --show-error --max-time 10 http://127.0.0.1:3200/readyz)"
[[ "$health" == *'"status":"ok"'* \
  && "$health" == *"\"release\":\"${release_sha}\""* ]] \
  || fail "production app liveness did not expose the tested release"
[[ "$ready" == *'"status":"ready"'* ]] \
  || fail "production app did not become Mongo-ready"

QUIZBATTLE_COMPOSE_FILE="$compose_file" \
QUIZBATTLE_ENV_FILE="$env_file" \
QUIZBATTLE_SECRET_DIR="$secret_dir" \
  "$fixture_dir/backup-mongo.sh"

mapfile -t backups < <(find /var/backups/quizbattle/mongo -maxdepth 1 -type f \
  -name 'quizbattle-mongo-*.archive.gz.p7m' -print)
[[ "${#backups[@]}" -eq 1 ]] || fail "the encrypted backup was not created exactly once"
backup_file="${backups[0]}"
(cd /var/backups/quizbattle/mongo && sha256sum --check "$(basename "${backup_file}.sha256")")

decrypted="$fixture_dir/decrypted.archive.gz"
openssl cms -decrypt -binary -inform DER \
  -in "$backup_file" \
  -recip "$secret_dir/backup-recipient.pem" \
  -inkey "$secret_dir/backup-private-key.pem" \
  -out "$decrypted"
[[ -s "$decrypted" ]] || fail "the decrypted Mongo archive is empty"
gzip --test "$decrypted" || fail "the decrypted Mongo archive is not valid gzip data"

tampered="$fixture_dir/tampered.p7m"
cp -- "$backup_file" "$tampered"
tamper_offset=$(( $(stat -c '%s' "$tampered") - 20 ))
((tamper_offset > 0)) || fail "the encrypted backup is unexpectedly small"
read -r original_byte < <(od -An -tu1 -j "$tamper_offset" -N1 "$tampered")
replacement_byte=$(( (original_byte + 1) % 256 ))
printf '%b' "\\$(printf '%03o' "$replacement_byte")" \
  | dd of="$tampered" bs=1 seek="$tamper_offset" conv=notrunc status=none
if openssl cms -decrypt -binary -inform DER \
  -in "$tampered" \
  -recip "$secret_dir/backup-recipient.pem" \
  -inkey "$secret_dir/backup-private-key.pem" \
  -out "$fixture_dir/tampered-output" >/dev/null 2>&1; then
  fail "tampering with the encrypted backup was not rejected"
fi

printf 'Fresh-volume production Compose, readiness, and encrypted-backup checks passed.\n'
