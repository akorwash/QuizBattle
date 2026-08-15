#!/usr/bin/env bash
# Contract probes below are intentionally single-quoted to test literal Compose
# and shell expansion syntax instead of expanding it in this test process.
# shellcheck disable=SC2016
set -Eeuo pipefail

deploy_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
compose_file="${deploy_dir}/docker-compose.production.yml"
deploy_file="${deploy_dir}/deploy.sh"
installer_file="${deploy_dir}/bootstrap.sh"
entrypoint_file="${deploy_dir}/ssh-entrypoint.sh"
bootstrap_file="${deploy_dir}/bootstrap-secrets.sh"
mongo_init_file="${deploy_dir}/mongo-init.sh"
backup_file="${deploy_dir}/backup-mongo.sh"
backup_service_file="${deploy_dir}/quizbattle-mongo-backup.service"
certbot_hook_file="${deploy_dir}/certbot-renew-hook.sh"
integration_file="${deploy_dir}/tests/test-production-compose.sh"
nginx_file="${deploy_dir}/nginx/quizbattle.qubefyn.com.conf"
example_file="${deploy_dir}/production.env.example"
workflow_file="${deploy_dir}/../.github/workflows/production.yml"

fail() {
  printf 'production asset test failed: %s\n' "$*" >&2
  exit 1
}

assert_fixed() {
  local needle="$1"
  local file="$2"
  grep -Fq -- "${needle}" "${file}" || fail "${file} does not contain: ${needle}"
}

for file in \
  "${compose_file}" "${deploy_file}" "${installer_file}" "${entrypoint_file}" \
  "${bootstrap_file}" "${mongo_init_file}" "${backup_file}" \
  "${backup_service_file}" "${certbot_hook_file}" "${integration_file}" \
  "${nginx_file}" "${example_file}" "${workflow_file}"; do
  [[ -s "${file}" ]] || fail "missing file: ${file}"
done

bash -n "${deploy_file}"
bash -n "${installer_file}"
bash -n "${entrypoint_file}"
bash -n "${bootstrap_file}"
bash -n "${mongo_init_file}"
bash -n "${backup_file}"
bash -n "${certbot_hook_file}"
bash -n "${integration_file}"

for command_name in openssl mktemp cmp stat od dd cp rm; do
  command -v "${command_name}" >/dev/null 2>&1 || fail "required test command is missing: ${command_name}"
done

cms_test_dir="$(mktemp -d)"
cleanup_cms_test() {
  rm -rf -- "${cms_test_dir}"
}
trap cleanup_cms_test EXIT
openssl genrsa -out "${cms_test_dir}/key.pem" 2048 >/dev/null 2>&1
openssl req -x509 -new -sha256 -days 1 \
  -key "${cms_test_dir}/key.pem" \
  -subj '/CN=QuizBattle deployment test' \
  -out "${cms_test_dir}/certificate.pem" >/dev/null 2>&1
printf 'authenticated-backup-test' >"${cms_test_dir}/plain"
openssl cms -encrypt -binary -stream -aes-256-gcm -outform DER \
  -out "${cms_test_dir}/backup.p7m" "${cms_test_dir}/certificate.pem" \
  <"${cms_test_dir}/plain"
openssl cms -decrypt -binary -inform DER \
  -in "${cms_test_dir}/backup.p7m" \
  -recip "${cms_test_dir}/certificate.pem" \
  -inkey "${cms_test_dir}/key.pem" \
  -out "${cms_test_dir}/decrypted"
cmp "${cms_test_dir}/plain" "${cms_test_dir}/decrypted" >/dev/null \
  || fail "AES-256-GCM CMS backup did not decrypt losslessly"

cp "${cms_test_dir}/backup.p7m" "${cms_test_dir}/tampered.p7m"
tamper_offset=$(( $(stat -c '%s' "${cms_test_dir}/tampered.p7m") - 20 ))
read -r original_byte < <(od -An -tu1 -j "${tamper_offset}" -N1 "${cms_test_dir}/tampered.p7m")
replacement_byte=$(( (original_byte + 1) % 256 ))
printf '%b' "\\$(printf '%03o' "${replacement_byte}")" \
  | dd of="${cms_test_dir}/tampered.p7m" bs=1 seek="${tamper_offset}" conv=notrunc status=none
if openssl cms -decrypt -binary -inform DER \
  -in "${cms_test_dir}/tampered.p7m" \
  -recip "${cms_test_dir}/certificate.pem" \
  -inkey "${cms_test_dir}/key.pem" \
  -out "${cms_test_dir}/tampered-output" >/dev/null 2>&1; then
  fail "AES-256-GCM CMS backup tampering was not rejected"
fi
cleanup_cms_test
trap - EXIT

assert_fixed 'name: quizbattle' "${compose_file}"
assert_fixed 'image: ${QUIZBATTLE_IMAGE:?QUIZBATTLE_IMAGE must be an immutable image@sha256 digest}' "${compose_file}"
assert_fixed 'mongo:8.0@sha256:de267922bc1153d923f5c9dc429f21c11faf18299080c1ce04d6d6007097fb06' "${compose_file}"
assert_fixed '127.0.0.1:${APP_PORT:-3200}:8080' "${compose_file}"
assert_fixed 'test: ["CMD", "/app/quizbattle", "healthcheck"]' "${compose_file}"
assert_fixed 'subnet: 172.30.8.0/24' "${compose_file}"
assert_fixed 'gateway: 172.30.8.1' "${compose_file}"
assert_fixed 'TRUSTED_PROXY_CIDRS: 172.30.8.1/32' "${compose_file}"
assert_fixed 'internal: true' "${compose_file}"
assert_fixed 'mem_limit: 1g' "${compose_file}"
assert_fixed 'mem_limit: 2g' "${compose_file}"
assert_fixed '--tlsMode=requireTLS' "${compose_file}"
assert_fixed '--tlsCAFile=/run/quizbattle-mongo/ca.crt' "${compose_file}"
assert_fixed 'MONGO_INITDB_ROOT_PASSWORD_FILE: /run/quizbattle-mongo/root-password' "${compose_file}"
assert_fixed '/run/secrets/mongo_root_password /run/quizbattle-mongo/root-password' "${compose_file}"
assert_fixed 'deploy:' "${compose_file}"
assert_fixed 'replicas: 1' "${compose_file}"

[[ "$(grep -c '^    ports:' "${compose_file}")" -eq 1 ]] || fail "only the app may publish a host port"
if grep -Eiq 'redis|tlsAllowInvalid|tlsInsecure|0\.0\.0\.0:' "${compose_file}"; then
  fail "Compose contains a forbidden Redis, insecure TLS, or public-bind setting"
fi

assert_fixed 'roles = [{ role: "readWrite", db: databaseName }]' "${mongo_init_file}"
assert_fixed 'existing replica-set topology does not match' "${mongo_init_file}"
assert_fixed 'ca_file="/run/secrets/mongo_ca_certificate"' "${mongo_init_file}"
assert_fixed '--oplog' "${backup_file}"
assert_fixed '--config "${config_file}"' "${backup_file}"
assert_fixed '--authenticationMechanism SCRAM-SHA-256' "${backup_file}"
assert_fixed '--sslCAFile /run/quizbattle-mongo/ca.crt' "${backup_file}"
assert_fixed '--archive --gzip --oplogReplay' "${backup_file}"
assert_fixed 'openssl cms -encrypt' "${backup_file}"
assert_fixed '-aes-256-gcm' "${backup_file}"
assert_fixed 'retention_days="${QUIZBATTLE_BACKUP_RETENTION_DAYS:-14}"' "${backup_file}"
assert_fixed 'backup_dir="/var/backups/quizbattle/mongo"' "${backup_file}"
assert_fixed 'lock_file="${backup_dir}/.backup.lock"' "${backup_file}"
assert_fixed "stat -c '%u:%g:%a:%h'" "${backup_file}"
assert_fixed 'readonly LOCK_FILE="$DEPLOY_DIR/.deploy.lock"' "${deploy_file}"
assert_fixed 'readonly DOCKER_CONFIG="$DEPLOY_DIR/.docker"' "${deploy_file}"
assert_fixed 'readonly MONGO_IMAGE="mongo:8.0@sha256:de267922bc1153d923f5c9dc429f21c11faf18299080c1ce04d6d6007097fb06"' "${deploy_file}"
assert_fixed 'readonly MONGO_REPO_DIGEST="mongo@sha256:de267922bc1153d923f5c9dc429f21c11faf18299080c1ce04d6d6007097fb06"' "${deploy_file}"
assert_fixed 'pull_and_verify_immutable_image "$MONGO_IMAGE" "$MONGO_REPO_DIGEST"' "${deploy_file}"
assert_fixed 'pull_and_verify_immutable_image "$target_image" "$target_image"' "${deploy_file}"
assert_fixed 'Docker did not retain the requested repository digest: ${expected_repo_digest}.' "${deploy_file}"
assert_fixed 'The previous image revision label does not match the previous release SHA.' "${deploy_file}"
assert_fixed "'^cf-ray:[[:space:]]*[[:graph:]]+'" "${deploy_file}"
assert_fixed 'readonly DOCKER_CONFIG_DIR="$DEPLOY_DIR/.docker"' "${installer_file}"
assert_fixed 'readonly DOCKER_CONFIG="$DEPLOY_DIR/.docker"' "${entrypoint_file}"
assert_fixed 'ReadWritePaths=/var/backups/quizbattle/mongo /run/docker.sock' "${backup_service_file}"
assert_fixed 'CERTBOT_HOOK_TARGET="$CERTBOT_HOOK_DIR/quizbattle-nginx-reload.sh"' "${installer_file}"
assert_fixed 'show_account_arguments=(show_account --non-interactive)' "${installer_file}"
assert_fixed 'certbot_arguments+=(--account "$certbot_account")' "${installer_file}"
assert_fixed 'must be exactly 32 lowercase hexadecimal characters' "${installer_file}"
assert_fixed 'timeout --signal=TERM --kill-after=5s 15s' "${installer_file}"
assert_fixed '750|/etc/letsencrypt/renewal-hooks/deploy/quizbattle-nginx-reload.sh' "${entrypoint_file}"
assert_fixed 'nginx -t' "${certbot_hook_file}"
assert_fixed 'systemctl reload nginx' "${certbot_hook_file}"
assert_fixed 'ssl_protocols TLSv1.2 TLSv1.3;' "${nginx_file}"
assert_fixed 'go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...' "${workflow_file}"
assert_fixed 'go run github.com/securego/gosec/v2/cmd/gosec@v2.28.0 -quiet ./...' "${workflow_file}"
assert_fixed 'fetch-depth: 0' "${workflow_file}"
assert_fixed 'zricethezav/gitleaks@sha256:aa036a2f4bdfe3cc3c55fa4326308efabb4a6be498c883c864fd1d0d5585438a' "${workflow_file}"
assert_fixed 'git /repo --redact=100 --no-banner' "${workflow_file}"
assert_fixed "'^cf-ray:[[:space:]]*[[:graph:]]+'" "${workflow_file}"
assert_fixed 'bash deploy/tests/test-production-compose.sh' "${workflow_file}"
assert_fixed 'deploy/certbot-renew-hook.sh' "${workflow_file}"
assert_fixed 'for command_name in openssl install stat grep awk' "${bootstrap_file}"
assert_fixed 'ghcr\.io/akorwash/quizbattle@sha256:' "${bootstrap_file}"
assert_fixed 'app port must remain exactly 3200' "${bootstrap_file}"
assert_fixed 'read_env_value()' "${bootstrap_file}"
assert_fixed 'fs.readFileSync("/run/secrets/mongo_root_password"' "${mongo_init_file}"
assert_fixed 'fs.readFileSync("/run/secrets/mongo_app_password"' "${mongo_init_file}"
assert_fixed 'QUIZBATTLE_ENV_FILE:-${script_dir}/.env' "${bootstrap_file}"
assert_fixed 'TRUSTED_PROXY_CIDRS:-172.30.8.1/32' "${bootstrap_file}"
assert_fixed 'TRUSTED_PROXY_CIDRS=172.30.8.1/32' "${example_file}"

mongo_pull_line="$(grep -nF \
  'pull_and_verify_immutable_image "$MONGO_IMAGE" "$MONGO_REPO_DIGEST"' \
  "${deploy_file}" | tail -n 1 | cut -d: -f1)"
app_pull_line="$(grep -nF \
  'pull_and_verify_immutable_image "$target_image" "$target_image"' \
  "${deploy_file}" | tail -n 1 | cut -d: -f1)"
backup_line="$(grep -nF '  "$BACKUP_SCRIPT"' "${deploy_file}" | head -n 1 | cut -d: -f1)"
[[ -n "${mongo_pull_line}" && -n "${app_pull_line}" && -n "${backup_line}" ]] \
  || fail "could not determine immutable-image preflight ordering"
(( mongo_pull_line < backup_line && app_pull_line < backup_line )) \
  || fail "all immutable images must be pulled and verified before the first deployment mutation"

if grep -Eq '(^|[[:space:]])set[[:space:]]+-x' "${bootstrap_file}" "${mongo_init_file}" "${backup_file}"; then
  fail "deployment scripts must never enable shell tracing around secrets"
fi

if grep -Eq '(^|[[:space:]])(source|\.)[[:space:]]+.*env' "${bootstrap_file}"; then
  fail "bootstrap must parse allowlisted environment keys without source/eval"
fi

if grep -Fq -- '--clusterAuthMode=keyFile' "${compose_file}"; then
  fail "explicit clusterAuthMode breaks the official Mongo first-start entrypoint"
fi

if grep -Fq -- '--password' "${compose_file}" "${mongo_init_file}" "${backup_file}"; then
  fail "database credentials must never be passed through process arguments"
fi

if grep -Fq -- '/run/lock' "${deploy_file}" "${backup_file}" "${backup_service_file}"; then
  fail "deployment and backup locks must stay inside reviewed root-only directories"
fi

if grep -Fq -- '700|/home/Qubefyn/QuizBattle/.docker' "${entrypoint_file}"; then
  fail "the runtime digest must not hash the Docker credential directory"
fi

if [[ -n "${QUIZBATTLE_TEST_ENV_FILE:-}" ]]; then
  docker compose \
    --project-name quizbattle \
    --env-file "${QUIZBATTLE_TEST_ENV_FILE}" \
    -f "${compose_file}" \
    config --quiet
fi

printf 'Production deployment asset checks passed.\n'
