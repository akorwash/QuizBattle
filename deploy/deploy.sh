#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

readonly DEPLOY_DIR="/home/Qubefyn/QuizBattle"
readonly COMPOSE_FILE="$DEPLOY_DIR/docker-compose.production.yml"
readonly ENV_FILE="$DEPLOY_DIR/.env"
readonly PREVIOUS_ENV_FILE="$DEPLOY_DIR/.env.previous"
readonly STATE_FILE="$DEPLOY_DIR/.last-success"
readonly LOCK_FILE="$DEPLOY_DIR/.deploy.lock"
readonly DOCKER_CONFIG="$DEPLOY_DIR/.docker"
readonly BACKUP_SCRIPT="$DEPLOY_DIR/backup-mongo.sh"
readonly APP_PORT="3200"
readonly PUBLIC_HEALTH_URL="https://quizbattle.qubefyn.com/healthz"
readonly MONGO_IMAGE="mongo:8.0@sha256:de267922bc1153d923f5c9dc429f21c11faf18299080c1ce04d6d6007097fb06"
readonly MONGO_REPO_DIGEST="mongo@sha256:de267922bc1153d923f5c9dc429f21c11faf18299080c1ce04d6d6007097fb06"
export DOCKER_CONFIG

log() {
  printf '[quizbattle-deploy] %s\n' "$*"
}

die() {
  log "ERROR: $*" >&2
  exit 64
}

trusted_root_file() {
  local path="$1"
  local mode="$2"
  [[ -f "$path" && ! -L "$path" ]] || return 1
  [[ "$(stat -c '%u:%g:%a' -- "$path")" == "0:0:${mode}" ]]
}

read_env_value() {
  local key="$1"
  awk -F= -v wanted="$key" '
    $1 == wanted { value = substr($0, index($0, "=") + 1) }
    END { print value }
  ' "$ENV_FILE"
}

write_release_env() {
  local image="$1"
  local release_sha="$2"
  local temporary_file

  temporary_file="$(mktemp "$DEPLOY_DIR/.env.release.XXXXXX")"
  if ! awk -v image="$image" -v release_sha="$release_sha" '
    BEGIN { found_image = 0; found_release = 0 }
    /^QUIZBATTLE_IMAGE=/ {
      print "QUIZBATTLE_IMAGE=" image
      found_image = 1
      next
    }
    /^RELEASE_SHA=/ {
      print "RELEASE_SHA=" release_sha
      found_release = 1
      next
    }
    { print }
    END {
      if (!found_image) print "QUIZBATTLE_IMAGE=" image
      if (!found_release) print "RELEASE_SHA=" release_sha
    }
  ' "$ENV_FILE" > "$temporary_file"; then
    rm -f -- "$temporary_file"
    return 1
  fi
  chmod 600 "$temporary_file"
  chown root:root "$temporary_file"
  mv -f -- "$temporary_file" "$ENV_FILE"
}

pull_and_verify_immutable_image() {
  local image="$1"
  local expected_repo_digest="$2"
  local repo_digest

  [[ "$image" == *@sha256:* && "$expected_repo_digest" == *@sha256:* ]] \
    || die "Refusing to pull an image that is not pinned by digest."

  log "Pulling immutable image ${image}."
  docker pull "$image"

  while IFS= read -r repo_digest; do
    if [[ "$repo_digest" == "$expected_repo_digest" ]]; then
      return 0
    fi
  done < <(docker image inspect \
    --format '{{range .RepoDigests}}{{println .}}{{end}}' "$image")

  die "Docker did not retain the requested repository digest: ${expected_repo_digest}."
}

verify_health() {
  local expected_release="$1"
  local response=""

  for _attempt in $(seq 1 60); do
    response="$(curl --fail --silent --show-error --max-time 5 \
      "http://127.0.0.1:${APP_PORT}/healthz" 2>/dev/null || true)"
    if [[ "$response" == *'"status":"ok"'* \
      && "$response" == *"\"release\":\"${expected_release}\""* ]]; then
      return 0
    fi
    sleep 2
  done
  log "Local liveness did not confirm release ${expected_release}."
  return 1
}

verify_readiness() {
  local response=""

  for _attempt in $(seq 1 60); do
    response="$(curl --fail --silent --show-error --max-time 5 \
      "http://127.0.0.1:${APP_PORT}/readyz" 2>/dev/null || true)"
    if [[ "$response" == *'"status":"ready"'* ]]; then
      return 0
    fi
    sleep 2
  done
  log "MongoDB readiness did not become healthy."
  return 1
}

verify_public_release() {
  local expected_release="$1"
  local body_file
  local headers_file
  local response=""

  headers_file="$(mktemp "$DEPLOY_DIR/.public-health.headers.XXXXXX")"
  body_file="$(mktemp "$DEPLOY_DIR/.public-health.body.XXXXXX")"

  for _attempt in $(seq 1 30); do
    if curl --fail --silent --show-error --max-time 10 \
      -H 'Cache-Control: no-cache' \
      --dump-header "$headers_file" \
      --output "$body_file" \
      "${PUBLIC_HEALTH_URL}?release=${expected_release}" 2>/dev/null; then
      response="$(<"$body_file")"
    else
      response=""
    fi
    if [[ "$response" == *'"status":"ok"'* \
      && "$response" == *"\"release\":\"${expected_release}\""* ]] \
      && grep --extended-regexp --ignore-case --quiet \
        '^cf-ray:[[:space:]]*[[:graph:]]+' "$headers_file"; then
      rm -f -- "$headers_file" "$body_file"
      return 0
    fi
    sleep 5
  done
  rm -f -- "$headers_file" "$body_file"
  log "Cloudflare did not expose release ${expected_release} with a cf-ray header."
  return 1
}

write_success_state() {
  local image="$1"
  local release_sha="$2"
  local run_id="$3"
  local temporary_file

  temporary_file="$(mktemp "$DEPLOY_DIR/.last-success.XXXXXX")"
  printf 'RUN_ID=%s\nRELEASE_SHA=%s\nQUIZBATTLE_IMAGE=%s\n' \
    "$run_id" "$release_sha" "$image" > "$temporary_file"
  chmod 600 "$temporary_file"
  chown root:root "$temporary_file"
  mv -f -- "$temporary_file" "$STATE_FILE"
}

target_image="${1:-}"
target_release="${2:-}"
target_run_id="${3:-}"

[[ "$target_image" =~ ^ghcr\.io/akorwash/quizbattle@sha256:[0-9a-f]{64}$ ]] \
  || die "The image must be an immutable QuizBattle GHCR digest."
[[ "$target_release" =~ ^[0-9a-f]{40}$ ]] || die "The release SHA is invalid."
[[ "$target_run_id" =~ ^[0-9]+$ ]] || die "The GitHub run id is invalid."

[[ "${EUID}" -eq 0 ]] || die "Deployment must run as root."
for command_name in curl docker flock readlink stat; do
  command -v "$command_name" >/dev/null || die "Required executable is unavailable: ${command_name}."
done
[[ -d "$DEPLOY_DIR" && ! -L "$DEPLOY_DIR" ]] || die "Deployment directory is unsafe or missing."
[[ "$(readlink -f -- "$DEPLOY_DIR")" == "$DEPLOY_DIR" \
  && "$(stat -c '%u:%g:%a' -- "$DEPLOY_DIR")" == "0:0:700" ]] \
  || die "Deployment directory must be root:root mode 0700 without symlink traversal."
[[ -d "$DOCKER_CONFIG" && ! -L "$DOCKER_CONFIG" \
  && "$(readlink -f -- "$DOCKER_CONFIG")" == "$DOCKER_CONFIG" \
  && "$(stat -c '%u:%g:%a' -- "$DOCKER_CONFIG")" == "0:0:700" ]] \
  || die "QuizBattle Docker credential directory is unsafe or missing."
trusted_root_file "$COMPOSE_FILE" 640 || die "Production Compose file ownership or mode is invalid."
trusted_root_file "$ENV_FILE" 600 || die "Production environment ownership or mode is invalid."
trusted_root_file "$BACKUP_SCRIPT" 750 || die "Mongo backup script ownership or mode is invalid."

if [[ -e "$LOCK_FILE" || -L "$LOCK_FILE" ]]; then
  [[ -f "$LOCK_FILE" && ! -L "$LOCK_FILE" ]] || die "Deployment lock must be a regular file."
else
  (set -o noclobber; umask 077; : > "$LOCK_FILE") \
    || die "Deployment lock could not be created safely."
fi
[[ "$(stat -c '%u:%g:%a:%h' -- "$LOCK_FILE")" == "0:0:600:1" ]] \
  || die "Deployment lock must be root:root mode 0600 with one link."
exec 9<>"$LOCK_FILE"
if ! flock -w 600 9; then
  die "Another deployment held the lock for more than ten minutes."
fi

last_run_id="0"
if [[ -f "$STATE_FILE" ]]; then
  trusted_root_file "$STATE_FILE" 600 || die "The last-success state is unsafe."
  last_run_id="$(awk -F= '$1 == "RUN_ID" { print $2 }' "$STATE_FILE" | tail -n 1)"
  [[ "$last_run_id" =~ ^[0-9]+$ ]] || last_run_id="0"
fi
if (( target_run_id < last_run_id )); then
  log "STALE_DEPLOY_SKIPPED target_run_id=${target_run_id} last_run_id=${last_run_id}"
  exit 0
fi

configured_port="$(read_env_value APP_PORT)"
[[ "$configured_port" == "$APP_PORT" ]] \
  || die "APP_PORT must remain ${APP_PORT}; Nginx is pinned to that loopback port."

previous_image="$(read_env_value QUIZBATTLE_IMAGE)"
previous_release="$(read_env_value RELEASE_SHA)"
rollback_available="false"
if [[ "$previous_release" == "bootstrap" ]]; then
  : # There is no application release to restore during the first deployment.
elif [[ "$previous_image" =~ ^ghcr\.io/akorwash/quizbattle@sha256:[0-9a-f]{64}$ \
  && "$previous_release" =~ ^[0-9a-f]{40}$ ]]; then
  docker image inspect "$previous_image" >/dev/null 2>&1 \
    || die "The previous image is unavailable locally; refusing a deployment without rollback capacity."
  previous_image_revision="$(docker image inspect \
    --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' \
    "$previous_image")" \
    || die "The previous image revision label could not be read."
  [[ "$previous_image_revision" == "$previous_release" ]] \
    || die "The previous image revision label does not match the previous release SHA."
  rollback_available="true"
else
  die "Previous release metadata is invalid."
fi

compose=(docker compose --project-name quizbattle --env-file "$ENV_FILE" -f "$COMPOSE_FILE")
"${compose[@]}" config --quiet

# Compose deliberately runs with --pull never so that startup cannot silently
# substitute a different image. Preload and verify every required immutable
# image before taking a backup or changing release state.
pull_and_verify_immutable_image "$MONGO_IMAGE" "$MONGO_REPO_DIGEST"
pull_and_verify_immutable_image "$target_image" "$target_image"
image_revision="$(docker image inspect \
  --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' \
  "$target_image")"
[[ "$image_revision" == "$target_release" ]] \
  || die "The image revision label does not match the release SHA."

# Fail closed before changing release state when a durable database already
# exists. A pristine bootstrap has no running Mongo service to snapshot yet.
if "${compose[@]}" ps --status running --services | grep --fixed-strings --line-regexp --quiet mongo; then
  log "Creating encrypted pre-deploy MongoDB snapshot."
  "$BACKUP_SCRIPT"
elif [[ "$previous_release" == "bootstrap" ]]; then
  log "First deployment has no running MongoDB; pre-deploy backup is not applicable."
else
  die "MongoDB is not running, so the required pre-deploy backup cannot be created."
fi

cp --preserve=mode,ownership -- "$ENV_FILE" "$PREVIOUS_ENV_FILE"
deployment_mutated="false"
deployment_committed="false"

rollback_release() {
  log "Rolling back to the previous deployment state."
  cp --preserve=mode,ownership -- "$PREVIOUS_ENV_FILE" "$ENV_FILE" || return $?
  if [[ "$rollback_available" == "true" ]]; then
    "${compose[@]}" up -d --no-build --pull never \
      --wait --wait-timeout 240 || return $?
    verify_health "$previous_release" || return $?
    verify_readiness || return $?
    verify_public_release "$previous_release" || return $?
    log "Rollback completed: ${previous_release}."
    return 0
  fi

  log "No previous application release exists; stopping only the failed app container."
  "${compose[@]}" stop -t 30 app
}

# Invoked indirectly by the HUP/TERM traps below.
# shellcheck disable=SC2317
handle_termination() {
  local signal_name="$1"
  local exit_status="$2"
  trap - HUP TERM
  log "Received ${signal_name}; terminating deployment safely." >&2 || true
  if [[ "$deployment_mutated" == "true" && "$deployment_committed" != "true" ]]; then
    rollback_release || log "ERROR: rollback after ${signal_name} failed." >&2
  fi
  exit "$exit_status"
}

trap 'handle_termination HUP 129' HUP
trap 'handle_termination TERM 143' TERM

deployment_mutated="true"
write_release_env "$target_image" "$target_release"
log "Deploying release ${target_release}."
if "${compose[@]}" up -d --no-build --pull never \
  --wait --wait-timeout 240 \
  && verify_health "$target_release" \
  && verify_readiness \
  && verify_public_release "$target_release"; then
  deferred_signal=""
  deferred_status="0"
  trap 'deferred_signal="HUP"; deferred_status="129"' HUP
  trap 'deferred_signal="TERM"; deferred_status="143"' TERM
  state_status=0
  write_success_state "$target_image" "$target_release" "$target_run_id" || state_status=$?
  if (( state_status == 0 )); then
    deployment_committed="true"
  fi
  trap 'handle_termination HUP 129' HUP
  trap 'handle_termination TERM 143' TERM
  if [[ -n "$deferred_signal" ]]; then
    handle_termination "$deferred_signal" "$deferred_status"
  fi
  if (( state_status == 0 )); then
    log "Deployment succeeded: ${target_release}."
    exit 0
  fi
  deploy_status="$state_status"
  log "Failed to persist the successful release state; rolling back." >&2
else
  deploy_status=$?
fi

log "Deployment failed with status ${deploy_status}; recent logs follow."
"${compose[@]}" logs --no-color --tail 160 mongo mongo-init app >&2 || true
if ! rollback_release; then
  log "ERROR: automatic rollback failed; operator intervention is required." >&2
fi
exit "$deploy_status"
