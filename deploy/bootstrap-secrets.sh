#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
secret_dir="${QUIZBATTLE_SECRET_DIR:-${script_dir}/secrets}"
env_file="${QUIZBATTLE_ENV_FILE:-${script_dir}/.env}"

image="${QUIZBATTLE_IMAGE:-}"
release_sha="${RELEASE_SHA:-}"
allowed_origins="${ALLOWED_ORIGINS:-}"
app_port="${APP_PORT:-3200}"
mongo_database="${MONGO_DATABASE:-quizbattle}"
mongo_root_username="${MONGO_ROOT_USERNAME:-quizbattle_root}"
mongo_app_username="${MONGO_APP_USERNAME:-quizbattle_app}"
trusted_proxy_cidrs="${TRUSTED_PROXY_CIDRS:-172.30.8.1/32}"
bootstrap_mode=false
readonly bootstrap_image="ghcr.io/akorwash/quizbattle@sha256:0000000000000000000000000000000000000000000000000000000000000000"

usage() {
  printf '%s\n' \
    "Usage: sudo $0 --bootstrap --origin https://quiz.example.com [options]" \
    "   or: sudo $0 --image ghcr.io/akorwash/quizbattle@sha256:DIGEST --release GIT_SHA --origin https://quiz.example.com [options]" \
    "" \
    "Options:" \
    "  --bootstrap           Create a first-release sentinel; do not supply image/release" \
    "  --app-port PORT       Host loopback port (must remain 3200)" \
    "  --database NAME       Mongo database (default: quizbattle)" \
    "  --env-file PATH       Destination environment file" \
    "  --secret-dir PATH     Destination for root-owned secrets" \
    "" \
    "Existing complete bootstrap material is validated and preserved."
}

fail() {
  printf 'bootstrap error: %s\n' "$*" >&2
  exit 1
}

while (($# > 0)); do
  case "$1" in
    --image)
      (($# >= 2)) || fail "--image requires a value"
      image="$2"
      shift 2
      ;;
    --bootstrap)
      bootstrap_mode=true
      shift
      ;;
    --release)
      (($# >= 2)) || fail "--release requires a value"
      release_sha="$2"
      shift 2
      ;;
    --origin)
      (($# >= 2)) || fail "--origin requires a value"
      allowed_origins="$2"
      shift 2
      ;;
    --app-port)
      (($# >= 2)) || fail "--app-port requires a value"
      app_port="$2"
      shift 2
      ;;
    --database)
      (($# >= 2)) || fail "--database requires a value"
      mongo_database="$2"
      shift 2
      ;;
    --env-file)
      (($# >= 2)) || fail "--env-file requires a value"
      env_file="$2"
      shift 2
      ;;
    --secret-dir)
      (($# >= 2)) || fail "--secret-dir requires a value"
      secret_dir="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

if [[ "${bootstrap_mode}" == true ]]; then
  [[ -z "${image}" && -z "${release_sha}" ]] \
    || fail "--bootstrap cannot be combined with --image, --release, or their environment variables"
  image="${bootstrap_image}"
  release_sha="bootstrap"
fi

[[ "${EUID}" -eq 0 ]] || fail "run this one-time bootstrap as root"
for command_name in openssl install stat grep awk mktemp tr cat rm; do
  command -v "${command_name}" >/dev/null 2>&1 || fail "required command is missing: ${command_name}"
done

assert_root_file() {
  local path="$1"
  local expected_mode="$2"
  [[ -f "${path}" && ! -L "${path}" ]] || fail "missing regular file: ${path}"
  [[ "$(stat -c '%u:%g' "${path}")" == "0:0" ]] || fail "file must be owned by root:root: ${path}"
  [[ "$(stat -c '%a' "${path}")" == "${expected_mode}" ]] || fail "file ${path} must have mode ${expected_mode}"
}

read_env_value() {
  local key="$1"
  awk -F= -v wanted="${key}" '
    $1 == wanted { value = substr($0, index($0, "=") + 1); found += 1 }
    END {
      if (found != 1) exit 2
      print value
    }
  ' "${env_file}"
}

validate_names_and_inputs() {
  if [[ "${bootstrap_mode}" == true ]]; then
    [[ "${image}" == "${bootstrap_image}" && "${release_sha}" == "bootstrap" ]] \
      || fail "invalid first-release bootstrap sentinel"
  else
    [[ "${image}" =~ ^ghcr\.io/akorwash/quizbattle@sha256:[a-f0-9]{64}$ ]] \
      || fail "image must be an immutable ghcr.io/akorwash/quizbattle digest"
    [[ "${release_sha}" =~ ^[a-f0-9]{40}$ ]] || fail "release must be a full 40-character lowercase Git SHA"
  fi
  [[ "${app_port}" == "3200" ]] || fail "app port must remain exactly 3200"
  [[ "${mongo_database}" =~ ^[A-Za-z0-9_-]{1,63}$ ]] || fail "database name contains unsupported characters"
  [[ "${mongo_root_username}" =~ ^[A-Za-z0-9_-]{1,63}$ ]] || fail "Mongo root username contains unsupported characters"
  [[ "${mongo_app_username}" =~ ^[A-Za-z0-9_-]{1,63}$ ]] || fail "Mongo app username contains unsupported characters"
  [[ -n "${allowed_origins}" && "${allowed_origins}" != *$'\n'* && "${allowed_origins}" != *' '* ]] || fail "at least one whitespace-free HTTPS origin is required"

  local origin
  local -a origins=()
  IFS=',' read -r -a origins <<<"${allowed_origins}"
  for origin in "${origins[@]}"; do
    [[ "${origin}" =~ ^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ ]] || fail "invalid production origin: ${origin}"
  done
  [[ "${trusted_proxy_cidrs}" == "172.30.8.1/32" ]] \
    || fail "trusted proxy CIDRs must remain exactly 172.30.8.1/32"
}

validate_certificates() {
  openssl verify -CAfile "${secret_dir}/mongo-ca.crt" "${secret_dir}/mongo-server.crt" >/dev/null
  openssl x509 -checkend 86400 -noout -in "${secret_dir}/mongo-ca.crt" >/dev/null || fail "Mongo CA certificate expires within 24 hours"
  openssl x509 -checkend 86400 -noout -in "${secret_dir}/mongo-server.crt" >/dev/null || fail "Mongo server certificate expires within 24 hours"
  openssl x509 -noout -ext subjectAltName -in "${secret_dir}/mongo-server.crt" | grep -q 'DNS:mongo' || fail "Mongo certificate SAN must contain DNS:mongo"
  openssl x509 -checkend 86400 -noout -in "${secret_dir}/backup-recipient.pem" >/dev/null || fail "backup recipient certificate expires within 24 hours"
}

validate_complete_bootstrap() {
  local configured_project
  local configured_image
  local configured_release
  local configured_environment
  local configured_mongo_uri
  local configured_cookie_secure
  local configured_port
  local configured_trusted_proxies

  assert_root_file "${env_file}" 600
  assert_root_file "${secret_dir}/mongo-root-password" 600
  assert_root_file "${secret_dir}/mongo-app-password" 600
  assert_root_file "${secret_dir}/jwt-secret" 600
  assert_root_file "${secret_dir}/mongo-keyfile" 600
  assert_root_file "${secret_dir}/mongo-ca.key" 600
  assert_root_file "${secret_dir}/mongo-ca.crt" 644
  assert_root_file "${secret_dir}/mongo-server.key" 600
  assert_root_file "${secret_dir}/mongo-server.crt" 644
  assert_root_file "${secret_dir}/mongo-server.pem" 600
  assert_root_file "${secret_dir}/backup-private-key.pem" 600
  assert_root_file "${secret_dir}/backup-recipient.pem" 644
  validate_certificates

  configured_project="$(read_env_value COMPOSE_PROJECT_NAME)" || fail ".env must contain COMPOSE_PROJECT_NAME exactly once"
  configured_image="$(read_env_value QUIZBATTLE_IMAGE)" || fail ".env must contain QUIZBATTLE_IMAGE exactly once"
  configured_release="$(read_env_value RELEASE_SHA)" || fail ".env must contain RELEASE_SHA exactly once"
  configured_environment="$(read_env_value APP_ENV)" || fail ".env must contain APP_ENV exactly once"
  configured_mongo_uri="$(read_env_value MONGO_URI)" || fail ".env must contain MONGO_URI exactly once"
  configured_cookie_secure="$(read_env_value COOKIE_SECURE)" || fail ".env must contain COOKIE_SECURE exactly once"
  configured_port="$(read_env_value APP_PORT)" || fail ".env must contain APP_PORT exactly once"
  configured_trusted_proxies="$(read_env_value TRUSTED_PROXY_CIDRS)" || fail ".env must contain TRUSTED_PROXY_CIDRS exactly once"

  [[ "${configured_project}" == "quizbattle" \
    && "${configured_image}" =~ ^ghcr\.io/akorwash/quizbattle@sha256:[a-f0-9]{64}$ \
    && ( "${configured_release}" =~ ^[a-f0-9]{40}$ \
      || ( "${configured_release}" == "bootstrap" \
        && "${configured_image}" == "${bootstrap_image}" ) ) \
    && "${configured_environment}" == "production" \
    && "${configured_mongo_uri}" == *'tls=true'* \
    && "${configured_mongo_uri}" == *'tlsCAFile=/run/quizbattle/tls/mongo-ca.crt'* \
    && "${configured_mongo_uri}" != *'tlsInsecure=true'* \
    && "${configured_cookie_secure}" == "true" \
    && "${configured_port}" == "3200" \
    && "${configured_trusted_proxies}" == "172.30.8.1/32" ]] \
    || fail ".env failed validation"
}

if [[ -e "${env_file}" ]]; then
  [[ -d "${secret_dir}" && ! -L "${secret_dir}" ]] || fail "secret directory is missing or unsafe"
  [[ "$(stat -c '%u:%g:%a' "${secret_dir}")" == "0:0:700" ]] || fail "secret directory must be root:root mode 0700"
  validate_complete_bootstrap
  printf 'QuizBattle bootstrap already exists and passed validation; nothing was changed.\n'
  exit 0
fi

validate_names_and_inputs

install -d -o root -g root -m 0700 "${secret_dir}"
[[ ! -L "${secret_dir}" ]] || fail "secret directory must not be a symlink"

generate_hex_secret() {
  local destination="$1"
  local bytes="$2"
  local temporary
  if [[ -e "${destination}" ]]; then
    assert_root_file "${destination}" 600
    return
  fi
  temporary="$(mktemp "${secret_dir}/.secret.XXXXXX")"
  openssl rand -hex "${bytes}" >"${temporary}"
  install -o root -g root -m 0600 "${temporary}" "${destination}"
  rm -f -- "${temporary}"
}

generate_hex_secret "${secret_dir}/mongo-root-password" 32
generate_hex_secret "${secret_dir}/mongo-app-password" 32
generate_hex_secret "${secret_dir}/jwt-secret" 48

if [[ ! -e "${secret_dir}/mongo-keyfile" ]]; then
  keyfile_temporary="$(mktemp "${secret_dir}/.keyfile.XXXXXX")"
  openssl rand -base64 512 | tr -d '\n' >"${keyfile_temporary}"
  printf '\n' >>"${keyfile_temporary}"
  install -o root -g root -m 0600 "${keyfile_temporary}" "${secret_dir}/mongo-keyfile"
  rm -f -- "${keyfile_temporary}"
else
  assert_root_file "${secret_dir}/mongo-keyfile" 600
fi

tls_names=(mongo-ca.key mongo-ca.crt mongo-server.key mongo-server.crt mongo-server.pem)
tls_existing=0
for tls_name in "${tls_names[@]}"; do
  [[ -e "${secret_dir}/${tls_name}" ]] && ((tls_existing += 1))
done
((tls_existing == 0 || tls_existing == ${#tls_names[@]})) || fail "partial Mongo TLS material exists; restore the complete set before retrying"

temporary_dir=""
cleanup() {
  [[ -z "${temporary_dir}" ]] || rm -rf -- "${temporary_dir}"
}
trap cleanup EXIT

if ((tls_existing == 0)); then
  temporary_dir="$(mktemp -d "${secret_dir}/.tls.XXXXXX")"
  openssl genrsa -out "${temporary_dir}/mongo-ca.key" 4096
  openssl req -x509 -new -sha256 -days 3650 \
    -key "${temporary_dir}/mongo-ca.key" \
    -subj '/CN=QuizBattle Mongo Internal CA' \
    -addext 'basicConstraints=critical,CA:TRUE,pathlen:0' \
    -addext 'keyUsage=critical,keyCertSign,cRLSign' \
    -out "${temporary_dir}/mongo-ca.crt"

  openssl genrsa -out "${temporary_dir}/mongo-server.key" 4096
  openssl req -new -sha256 \
    -key "${temporary_dir}/mongo-server.key" \
    -subj '/CN=mongo' \
    -addext 'subjectAltName=DNS:mongo,DNS:localhost,IP:127.0.0.1' \
    -addext 'basicConstraints=critical,CA:FALSE' \
    -addext 'keyUsage=critical,digitalSignature,keyEncipherment' \
    -addext 'extendedKeyUsage=serverAuth' \
    -out "${temporary_dir}/mongo-server.csr"
  openssl x509 -req -sha256 -days 825 \
    -in "${temporary_dir}/mongo-server.csr" \
    -CA "${temporary_dir}/mongo-ca.crt" \
    -CAkey "${temporary_dir}/mongo-ca.key" \
    -CAcreateserial \
    -copy_extensions copy \
    -out "${temporary_dir}/mongo-server.crt"
  cat "${temporary_dir}/mongo-server.key" "${temporary_dir}/mongo-server.crt" \
    >"${temporary_dir}/mongo-server.pem"

  install -o root -g root -m 0600 "${temporary_dir}/mongo-ca.key" "${secret_dir}/mongo-ca.key"
  install -o root -g root -m 0644 "${temporary_dir}/mongo-ca.crt" "${secret_dir}/mongo-ca.crt"
  install -o root -g root -m 0600 "${temporary_dir}/mongo-server.key" "${secret_dir}/mongo-server.key"
  install -o root -g root -m 0644 "${temporary_dir}/mongo-server.crt" "${secret_dir}/mongo-server.crt"
  install -o root -g root -m 0600 "${temporary_dir}/mongo-server.pem" "${secret_dir}/mongo-server.pem"
  rm -rf -- "${temporary_dir}"
  temporary_dir=""
fi

backup_names=(backup-private-key.pem backup-recipient.pem)
backup_existing=0
for backup_name in "${backup_names[@]}"; do
  [[ -e "${secret_dir}/${backup_name}" ]] && ((backup_existing += 1))
done
((backup_existing == 0 || backup_existing == ${#backup_names[@]})) || fail "partial backup encryption material exists; restore the complete pair before retrying"

if ((backup_existing == 0)); then
  temporary_dir="$(mktemp -d "${secret_dir}/.backup-cert.XXXXXX")"
  openssl genrsa -out "${temporary_dir}/backup-private-key.pem" 4096
  openssl req -x509 -new -sha256 -days 3650 \
    -key "${temporary_dir}/backup-private-key.pem" \
    -subj '/CN=QuizBattle Mongo Backup Recipient' \
    -addext 'basicConstraints=critical,CA:FALSE' \
    -addext 'keyUsage=critical,keyEncipherment,dataEncipherment' \
    -out "${temporary_dir}/backup-recipient.pem"
  install -o root -g root -m 0600 "${temporary_dir}/backup-private-key.pem" "${secret_dir}/backup-private-key.pem"
  install -o root -g root -m 0644 "${temporary_dir}/backup-recipient.pem" "${secret_dir}/backup-recipient.pem"
  rm -rf -- "${temporary_dir}"
  temporary_dir=""
fi

validate_certificates

mongo_app_password="$(<"${secret_dir}/mongo-app-password")"
jwt_secret="$(<"${secret_dir}/jwt-secret")"
mongo_uri="mongodb://${mongo_app_username}:${mongo_app_password}@mongo:27017/${mongo_database}?authSource=${mongo_database}&authMechanism=SCRAM-SHA-256&replicaSet=rs0&tls=true&tlsCAFile=/run/quizbattle/tls/mongo-ca.crt"
env_temporary="$(mktemp "${script_dir}/.env.XXXXXX")"
{
  printf 'COMPOSE_PROJECT_NAME=quizbattle\n'
  printf 'QUIZBATTLE_IMAGE=%s\n' "${image}"
  printf 'RELEASE_SHA=%s\n' "${release_sha}"
  printf 'APP_PORT=%s\n' "${app_port}"
  printf 'APP_ENV=production\n'
  printf 'MONGO_REPLICA_SET=rs0\n'
  printf 'MONGO_ROOT_USERNAME=%s\n' "${mongo_root_username}"
  printf 'MONGO_APP_USERNAME=%s\n' "${mongo_app_username}"
  printf 'MONGO_DATABASE=%s\n' "${mongo_database}"
  printf 'MONGO_URI=%s\n' "${mongo_uri}"
  printf 'JWT_SECRET=%s\n' "${jwt_secret}"
  printf 'SESSION_TTL=1h\n'
  printf 'COOKIE_SECURE=true\n'
  printf 'ALLOWED_ORIGINS=%s\n' "${allowed_origins}"
  printf 'TRUSTED_PROXY_CIDRS=%s\n' "${trusted_proxy_cidrs}"
  printf 'SEED_DATABASE=true\n'
} >"${env_temporary}"
install -o root -g root -m 0600 "${env_temporary}" "${env_file}"
rm -f -- "${env_temporary}"
unset mongo_app_password jwt_secret mongo_uri

validate_complete_bootstrap
printf 'QuizBattle production secrets and environment were created with strict root-only permissions.\n'
printf 'Environment: %s\nSecrets: %s\n' "${env_file}" "${secret_dir}"
printf 'Store an offline copy of backup-private-key.pem; it is required to decrypt database backups.\n'
