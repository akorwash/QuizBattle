#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

readonly DEPLOY_DIR="/home/Qubefyn/QuizBattle"
readonly DOCKER_CONFIG_DIR="$DEPLOY_DIR/.docker"
readonly NGINX_TARGET="/etc/nginx/sites-enabled/quizbattle.qubefyn.com"
readonly CERTIFICATE_DIR="/etc/letsencrypt/live/quizbattle.qubefyn.com"
readonly CERTBOT_HOOK_DIR="/etc/letsencrypt/renewal-hooks/deploy"
readonly CERTBOT_HOOK_TARGET="$CERTBOT_HOOK_DIR/quizbattle-nginx-reload.sh"
readonly ACME_ROOT="/var/www/quizbattle-acme"
readonly AUTHORIZED_KEYS="/root/.ssh/authorized_keys"
readonly FORCED_COMMAND="$DEPLOY_DIR/ssh-entrypoint.sh"

usage() {
  cat <<'EOF'
Usage: bootstrap.sh --deploy-public-key-file PATH [--certbot-account ID] [--letsencrypt-email ADDRESS]

Installs the reviewed QuizBattle runtime and a forced-command GitHub Actions key.
Select --certbot-account when the host has multiple Let's Encrypt accounts.
The email is required only when no selectable Let's Encrypt account exists.
EOF
}

public_key_file=""
certbot_account=""
letsencrypt_email=""
while (($# > 0)); do
  case "$1" in
    --deploy-public-key-file)
      (($# >= 2)) || { usage >&2; exit 64; }
      public_key_file="$2"
      shift 2
      ;;
    --letsencrypt-email)
      (($# >= 2)) || { usage >&2; exit 64; }
      letsencrypt_email="$2"
      shift 2
      ;;
    --certbot-account)
      (($# >= 2)) || { usage >&2; exit 64; }
      certbot_account="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 64
      ;;
  esac
done

[[ "${EUID}" -eq 0 ]] || { echo "bootstrap.sh must run as root." >&2; exit 77; }
if [[ -n "$certbot_account" && ! "$certbot_account" =~ ^[0-9a-f]{32}$ ]]; then
  echo "The Certbot account id must be exactly 32 lowercase hexadecimal characters." >&2
  exit 64
fi
[[ -n "$public_key_file" && -f "$public_key_file" && ! -L "$public_key_file" ]] \
  || { echo "A regular deploy public-key file is required." >&2; exit 66; }
if [[ -n "$letsencrypt_email" \
  && ! "$letsencrypt_email" =~ ^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$ ]]; then
  echo "The Let's Encrypt email address is invalid." >&2
  exit 64
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd -- "$script_dir/.." && pwd -P)"
compose_source="$repo_root/deploy/docker-compose.production.yml"
deploy_source="$repo_root/deploy/deploy.sh"
entrypoint_source="$repo_root/deploy/ssh-entrypoint.sh"
secret_bootstrap_source="$repo_root/deploy/bootstrap-secrets.sh"
mongo_init_source="$repo_root/deploy/mongo-init.sh"
backup_source="$repo_root/deploy/backup-mongo.sh"
backup_service_source="$repo_root/deploy/quizbattle-mongo-backup.service"
backup_timer_source="$repo_root/deploy/quizbattle-mongo-backup.timer"
certbot_hook_source="$repo_root/deploy/certbot-renew-hook.sh"
nginx_source="$repo_root/deploy/nginx/quizbattle.qubefyn.com.conf"

for source_file in \
  "$compose_source" "$deploy_source" "$entrypoint_source" \
  "$secret_bootstrap_source" "$mongo_init_source" "$backup_source" \
  "$backup_service_source" "$backup_timer_source" "$certbot_hook_source" \
  "$nginx_source"; do
  [[ -f "$source_file" && ! -L "$source_file" ]] \
    || { echo "Missing or unsafe reviewed file: $source_file" >&2; exit 66; }
done
for executable in docker nginx systemctl install readlink stat ssh-keygen timeout; do
  command -v "$executable" >/dev/null \
    || { echo "Required executable is unavailable: $executable" >&2; exit 69; }
done

if [[ ( -e "$DEPLOY_DIR" || -L "$DEPLOY_DIR" ) \
  && ( ! -d "$DEPLOY_DIR" || -L "$DEPLOY_DIR" ) ]]; then
  echo "The deployment directory must be a real directory." >&2
  exit 77
fi
install -d -o root -g root -m 0700 "$DEPLOY_DIR"
[[ "$(readlink -f -- "$DEPLOY_DIR")" == "$DEPLOY_DIR" \
  && "$(stat -c '%u:%g:%a' -- "$DEPLOY_DIR")" == "0:0:700" ]] \
  || { echo "The deployment directory is unsafe." >&2; exit 77; }
if [[ ( -e "$DOCKER_CONFIG_DIR" || -L "$DOCKER_CONFIG_DIR" ) \
  && ( ! -d "$DOCKER_CONFIG_DIR" || -L "$DOCKER_CONFIG_DIR" ) ]]; then
  echo "The QuizBattle Docker credential path must be a real directory." >&2
  exit 77
fi
install -d -o root -g root -m 0700 "$DOCKER_CONFIG_DIR"
[[ "$(readlink -f -- "$DOCKER_CONFIG_DIR")" == "$DOCKER_CONFIG_DIR" \
  && "$(stat -c '%u:%g:%a' -- "$DOCKER_CONFIG_DIR")" == "0:0:700" ]] \
  || { echo "The QuizBattle Docker credential directory is unsafe." >&2; exit 77; }
install -o root -g root -m 0640 "$compose_source" "$DEPLOY_DIR/docker-compose.production.yml"
install -o root -g root -m 0750 "$deploy_source" "$DEPLOY_DIR/deploy.sh"
install -o root -g root -m 0750 "$entrypoint_source" "$DEPLOY_DIR/ssh-entrypoint.sh"
install -o root -g root -m 0750 "$secret_bootstrap_source" "$DEPLOY_DIR/bootstrap-secrets.sh"
install -o root -g root -m 0750 "$mongo_init_source" "$DEPLOY_DIR/mongo-init.sh"
install -o root -g root -m 0750 "$backup_source" "$DEPLOY_DIR/backup-mongo.sh"
install -o root -g root -m 0644 "$backup_service_source" /etc/systemd/system/quizbattle-mongo-backup.service
install -o root -g root -m 0644 "$backup_timer_source" /etc/systemd/system/quizbattle-mongo-backup.timer
if [[ ( -e "$CERTBOT_HOOK_DIR" || -L "$CERTBOT_HOOK_DIR" ) \
  && ( ! -d "$CERTBOT_HOOK_DIR" || -L "$CERTBOT_HOOK_DIR" ) ]]; then
  echo "The Certbot deploy-hook path must be a real directory." >&2
  exit 77
fi
install -d -o root -g root -m 0755 "$CERTBOT_HOOK_DIR"
[[ "$(readlink -f -- "$CERTBOT_HOOK_DIR")" == "$CERTBOT_HOOK_DIR" \
  && "$(stat -c '%u:%g:%a' -- "$CERTBOT_HOOK_DIR")" == "0:0:755" ]] \
  || { echo "The Certbot deploy-hook directory is unsafe." >&2; exit 77; }
if [[ -e "$CERTBOT_HOOK_TARGET" || -L "$CERTBOT_HOOK_TARGET" ]]; then
  [[ -f "$CERTBOT_HOOK_TARGET" && ! -L "$CERTBOT_HOOK_TARGET" \
    && "$(stat -c '%u:%g:%a:%h' -- "$CERTBOT_HOOK_TARGET")" == "0:0:750:1" ]] \
    || { echo "The existing Certbot deploy hook is unsafe." >&2; exit 77; }
fi
install -o root -g root -m 0750 "$certbot_hook_source" "$CERTBOT_HOOK_TARGET"
[[ -f "$CERTBOT_HOOK_TARGET" && ! -L "$CERTBOT_HOOK_TARGET" \
  && "$(stat -c '%u:%g:%a:%h' -- "$CERTBOT_HOOK_TARGET")" == "0:0:750:1" ]] \
  || { echo "The installed Certbot deploy hook is unsafe." >&2; exit 77; }
install -d -o root -g root -m 0700 /var/backups/quizbattle/mongo

if [[ -e "$DEPLOY_DIR/.env" && ( -L "$DEPLOY_DIR/.env" \
  || "$(stat -c '%u:%g:%a' -- "$DEPLOY_DIR/.env")" != "0:0:600" ) ]]; then
  echo "The existing production .env has unsafe ownership, mode, or type." >&2
  exit 77
fi

install -d -o root -g root -m 0700 /root/.ssh
if [[ -e "$AUTHORIZED_KEYS" ]]; then
  [[ -f "$AUTHORIZED_KEYS" && ! -L "$AUTHORIZED_KEYS" ]] \
    || { echo "The root authorized_keys path is unsafe." >&2; exit 77; }
  chown root:root "$AUTHORIZED_KEYS"
  chmod 0600 "$AUTHORIZED_KEYS"
else
  install -o root -g root -m 0600 /dev/null "$AUTHORIZED_KEYS"
fi
[[ "$(grep --count --invert-match '^[[:space:]]*$' "$public_key_file")" == "1" ]] \
  || { echo "The deploy public-key file must contain exactly one key." >&2; exit 65; }
read -r key_type key_material _comment < "$public_key_file"
[[ "$key_type" == "ssh-ed25519" && "$key_material" =~ ^[A-Za-z0-9+/]+={0,3}$ ]] \
  || { echo "Only one valid ED25519 deploy public key is accepted." >&2; exit 65; }
key_check="$(mktemp)"
printf '%s %s\n' "$key_type" "$key_material" > "$key_check"
if ! ssh-keygen -lf "$key_check" >/dev/null 2>&1; then
  rm -f -- "$key_check"
  echo "The deploy public key could not be parsed." >&2
  exit 65
fi
rm -f -- "$key_check"
forced_key_line="restrict,command=\"${FORCED_COMMAND}\" ${key_type} ${key_material} quizbattle-github-actions"
matching_key_lines="$(grep --fixed-strings --count "$key_material" "$AUTHORIZED_KEYS" || true)"
matching_forced_lines="$(grep --fixed-strings --line-regexp --count "$forced_key_line" "$AUTHORIZED_KEYS" || true)"
if [[ "$matching_key_lines" == "0" ]]; then
  printf '%s\n' "$forced_key_line" >> "$AUTHORIZED_KEYS"
elif [[ "$matching_key_lines" != "1" || "$matching_forced_lines" != "1" ]]; then
  echo "The deploy key must appear exactly once with the expected restrictions." >&2
  exit 77
fi

# Qubefyn.Site already installs this shared global trust boundary. Refuse to
# deploy behind Cloudflare if it is absent rather than trusting spoofable headers.
cloudflare_real_ip="/etc/nginx/conf.d/cloudflare-real-ip.conf"
[[ -f "$cloudflare_real_ip" && ! -L "$cloudflare_real_ip" \
  && "$(stat -c '%u:%g:%a' -- "$cloudflare_real_ip")" == "0:0:644" ]] \
  || { echo "The reviewed Cloudflare real-IP include is missing or unsafe." >&2; exit 77; }

install -d -o root -g root -m 0755 "$ACME_ROOT"
install -d -o root -g root -m 0700 /etc/nginx/backups
nginx_backup=""
target_existed="false"
if [[ -e "$NGINX_TARGET" ]]; then
  [[ -f "$NGINX_TARGET" && ! -L "$NGINX_TARGET" ]] \
    || { echo "The existing QuizBattle Nginx vhost is unsafe." >&2; exit 77; }
  target_existed="true"
  nginx_backup="/etc/nginx/backups/quizbattle.$(date -u +%Y%m%dT%H%M%SZ).bak"
  cp --preserve=mode,ownership,timestamps -- "$NGINX_TARGET" "$nginx_backup"
fi

restore_nginx() {
  if [[ "$target_existed" == "true" ]]; then
    cp --preserve=mode,ownership,timestamps -- "$nginx_backup" "$NGINX_TARGET"
  else
    rm -f -- "$NGINX_TARGET"
  fi
  nginx -t >/dev/null 2>&1 && systemctl reload nginx || true
}

if [[ ! -f "$CERTIFICATE_DIR/fullchain.pem" || ! -f "$CERTIFICATE_DIR/privkey.pem" ]]; then
  command -v certbot >/dev/null \
    || { echo "certbot is required to obtain the origin certificate." >&2; exit 69; }
  certbot_account_exists="false"
  show_account_arguments=(show_account --non-interactive)
  if [[ -n "$certbot_account" ]]; then
    show_account_arguments+=(--account "$certbot_account")
  fi
  if timeout --signal=TERM --kill-after=5s 15s \
    certbot "${show_account_arguments[@]}" >/dev/null 2>&1; then
    certbot_account_exists="true"
  elif [[ -n "$certbot_account" ]]; then
    echo "The selected Certbot account is unavailable or could not be validated." >&2
    exit 66
  fi
  [[ -n "$letsencrypt_email" || "$certbot_account_exists" == "true" ]] \
    || { echo "--letsencrypt-email is required when Certbot has no existing account." >&2; exit 64; }
  temporary_vhost="$(mktemp)"
  cat > "$temporary_vhost" <<EOF
server {
    listen 80;
    listen [::]:80;
    server_name quizbattle.qubefyn.com;
    location ^~ /.well-known/acme-challenge/ {
        root ${ACME_ROOT};
        default_type text/plain;
    }
    location / { return 503; }
}
EOF
  install -o root -g root -m 0644 "$temporary_vhost" "$NGINX_TARGET"
  rm -f -- "$temporary_vhost"
  if ! nginx -t || ! systemctl reload nginx; then
    restore_nginx
    echo "The temporary ACME vhost failed validation." >&2
    exit 78
  fi
  certbot_arguments=(
    certonly
    --webroot
    --webroot-path "$ACME_ROOT"
    --domain quizbattle.qubefyn.com
    --agree-tos
    --non-interactive
  )
  if [[ -n "$letsencrypt_email" ]]; then
    certbot_arguments+=(--email "$letsencrypt_email")
  fi
  if [[ -n "$certbot_account" ]]; then
    certbot_arguments+=(--account "$certbot_account")
  fi
  if ! timeout --signal=TERM --kill-after=15s 300s \
    certbot "${certbot_arguments[@]}"; then
    restore_nginx
    echo "Let's Encrypt certificate issuance failed." >&2
    exit 75
  fi
fi

install -o root -g root -m 0644 "$nginx_source" "$NGINX_TARGET"
if ! nginx -t || ! systemctl reload nginx; then
  restore_nginx
  echo "The reviewed QuizBattle vhost failed; the prior Nginx state was restored." >&2
  exit 78
fi

systemctl daemon-reload
if [[ -e "$DEPLOY_DIR/.env" ]]; then
  systemctl enable --now quizbattle-mongo-backup.timer
else
  printf 'Enable quizbattle-mongo-backup.timer after bootstrap-secrets creates .env.\n'
fi

printf 'QuizBattle runtime installed at %s.\n' "$DEPLOY_DIR"
if [[ ! -e "$DEPLOY_DIR/.env" ]]; then
  printf 'Create the first-release environment with: %s --bootstrap --origin https://quizbattle.qubefyn.com\n' \
    "$DEPLOY_DIR/bootstrap-secrets.sh"
fi
printf 'Nginx is ready for quizbattle.qubefyn.com; enable Cloudflare proxy only with SSL Full (strict).\n'
