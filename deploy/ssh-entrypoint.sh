#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

readonly DEPLOY_DIR="/home/Qubefyn/QuizBattle"
readonly DEPLOY_SCRIPT="$DEPLOY_DIR/deploy.sh"
readonly COMPOSE_FILE="$DEPLOY_DIR/docker-compose.production.yml"
readonly ENV_FILE="$DEPLOY_DIR/.env"
readonly DOCKER_CONFIG="$DEPLOY_DIR/.docker"
export DOCKER_CONFIG

trusted_root_file() {
  local path="$1"
  local mode="$2"
  [[ -f "$path" && ! -L "$path" ]] || return 1
  [[ "$(stat -c '%u:%g:%a' -- "$path")" == "0:0:${mode}" ]]
}

trusted_root_directory() {
  local path="$1"
  local mode="$2"
  [[ -d "$path" && ! -L "$path" ]] || return 1
  [[ "$(readlink -f -- "$path")" == "$path" ]] || return 1
  [[ "$(stat -c '%u:%g:%a' -- "$path")" == "0:0:${mode}" ]]
}

runtime_digest() {
  local files=(
    "640|/home/Qubefyn/QuizBattle/docker-compose.production.yml"
    "750|/home/Qubefyn/QuizBattle/deploy.sh"
    "750|/home/Qubefyn/QuizBattle/ssh-entrypoint.sh"
    "750|/home/Qubefyn/QuizBattle/bootstrap-secrets.sh"
    "750|/home/Qubefyn/QuizBattle/mongo-init.sh"
    "750|/home/Qubefyn/QuizBattle/backup-mongo.sh"
    "644|/etc/systemd/system/quizbattle-mongo-backup.service"
    "644|/etc/systemd/system/quizbattle-mongo-backup.timer"
    "750|/etc/letsencrypt/renewal-hooks/deploy/quizbattle-nginx-reload.sh"
    "644|/etc/nginx/sites-enabled/quizbattle.qubefyn.com"
  )
  local expected_mode
  local path
  local specification

  for specification in "${files[@]}"; do
    expected_mode="${specification%%|*}"
    path="${specification#*|}"
    trusted_root_file "$path" "$expected_mode" || return 77
    sha256sum "$path" | awk '{ print $1 }'
  done | sha256sum | awk '{ print $1 }'
}

trusted_root_directory "$DEPLOY_DIR" 700 || exit 77
trusted_root_directory "$DOCKER_CONFIG" 700 || exit 77

original_command="${SSH_ORIGINAL_COMMAND:-}"
read -r action argument_one argument_two argument_three extra <<< "$original_command"

case "$action" in
  login-ghcr)
    [[ -n "$argument_one" && -z "$argument_two" && -z "$argument_three" && -z "$extra" ]] || exit 64
    [[ "$argument_one" =~ ^[A-Za-z0-9][A-Za-z0-9-]{0,38}$ ]] || exit 64
    exec docker login ghcr.io --username "$argument_one" --password-stdin
    ;;
  deploy)
    [[ -n "$argument_one" && -n "$argument_two" && -n "$argument_three" && -z "$extra" ]] || exit 64
    trusted_root_file "$DEPLOY_SCRIPT" 750 || exit 77
    exec "$DEPLOY_SCRIPT" "$argument_one" "$argument_two" "$argument_three"
    ;;
  logout-ghcr)
    [[ -z "$argument_one" && -z "$argument_two" && -z "$argument_three" && -z "$extra" ]] || exit 64
    exec docker logout ghcr.io
    ;;
  status)
    [[ -z "$argument_one" && -z "$argument_two" && -z "$argument_three" && -z "$extra" ]] || exit 64
    trusted_root_file "$COMPOSE_FILE" 640 || exit 77
    trusted_root_file "$ENV_FILE" 600 || exit 77
    exec docker compose --project-name quizbattle --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps
    ;;
  runtime-digest)
    [[ -z "$argument_one" && -z "$argument_two" && -z "$argument_three" && -z "$extra" ]] || exit 64
    runtime_digest
    ;;
  *)
    printf 'This SSH key is restricted to QuizBattle deployment commands.\n' >&2
    exit 64
    ;;
esac
