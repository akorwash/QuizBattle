#!/usr/bin/env bash
# Mongosh programs intentionally keep process.env expressions literal so they
# are evaluated inside the container instead of by this shell.
# shellcheck disable=SC2016
set -Eeuo pipefail

umask 077

fail() {
  printf 'mongo-init error: %s\n' "$*" >&2
  exit 1
}

assert_secret_file() {
  local path="$1"
  [[ -f "${path}" && -s "${path}" && ! -L "${path}" ]] || fail "required secret is missing"
}

mongo_host="${MONGO_HOST:-mongo}"
mongo_port="${MONGO_PORT:-27017}"
replica_set="${MONGO_REPLICA_SET:-rs0}"
root_username="${MONGO_ROOT_USERNAME:-quizbattle_root}"
app_username="${MONGO_APP_USERNAME:-quizbattle_app}"
database_name="${MONGO_DATABASE:-quizbattle}"
ca_file="/run/secrets/mongo_ca_certificate"
root_password_file="/run/secrets/mongo_root_password"
app_password_file="/run/secrets/mongo_app_password"

[[ "${mongo_host}" =~ ^[A-Za-z0-9.-]+$ ]] || fail "invalid Mongo hostname"
[[ "${mongo_port}" =~ ^[0-9]{1,5}$ ]] || fail "invalid Mongo port"
((mongo_port >= 1 && mongo_port <= 65535)) || fail "invalid Mongo port"
[[ "${replica_set}" =~ ^[A-Za-z0-9_-]{1,63}$ ]] || fail "invalid replica-set name"
[[ "${root_username}" =~ ^[A-Za-z0-9_-]{1,63}$ ]] || fail "invalid root username"
[[ "${app_username}" =~ ^[A-Za-z0-9_-]{1,63}$ ]] || fail "invalid app username"
[[ "${database_name}" =~ ^[A-Za-z0-9_-]{1,63}$ ]] || fail "invalid database name"
[[ -f "${ca_file}" && -s "${ca_file}" ]] || fail "Mongo CA certificate is unavailable"
assert_secret_file "${root_password_file}"
assert_secret_file "${app_password_file}"

connection="mongodb://${mongo_host}:${mongo_port}/admin?directConnection=true"
mongo=(
  mongosh
  "${connection}"
  --quiet
  --tls
  --tlsCAFile "${ca_file}"
)

auth_prelude='
  const fs = require("fs");
  const rootPassword = fs.readFileSync("/run/secrets/mongo_root_password", "utf8").trim();
  if (!rootPassword) throw new Error("Mongo root secret is empty");
  const adminDatabase = db.getSiblingDB("admin");
  const authenticated = adminDatabase.auth(process.env.MONGO_ROOT_USERNAME, rootPassword);
  if (!authenticated) throw new Error("Mongo root authentication failed");
'

run_mongo() {
  local javascript="$1"
  "${mongo[@]}" --eval "${auth_prelude}
${javascript}"
}

connected=false
for _ in $(seq 1 30); do
  if run_mongo 'quit(adminDatabase.runCommand({ping: 1}).ok === 1 ? 0 : 2)' >/dev/null 2>&1; then
    connected=true
    break
  fi
  sleep 2
done
[[ "${connected}" == true ]] || fail "Mongo did not accept an authenticated TLS connection in time"

run_mongo '
    const replicaSet = process.env.MONGO_REPLICA_SET;
    const memberHost = `${process.env.MONGO_HOST}:${process.env.MONGO_PORT}`;
    let initialized = true;
    try {
      rs.conf();
    } catch (error) {
      if (error.code === 94 || error.codeName === "NotYetInitialized") {
        initialized = false;
      } else {
        throw error;
      }
    }
    if (!initialized) {
      const result = rs.initiate({
        _id: replicaSet,
        members: [{ _id: 0, host: memberHost }]
      });
      if (!result.ok) throw new Error("replica-set initiation failed");
    }
  ' >/dev/null

primary=false
for _ in $(seq 1 45); do
  if run_mongo 'const hello = adminDatabase.hello(); quit(hello.isWritablePrimary ? 0 : 3)' >/dev/null 2>&1; then
    primary=true
    break
  fi
  sleep 2
done
[[ "${primary}" == true ]] || fail "Mongo replica set did not elect a primary in time"

run_mongo '
    const expectedSet = process.env.MONGO_REPLICA_SET;
    const expectedHost = `${process.env.MONGO_HOST}:${process.env.MONGO_PORT}`;
    const config = rs.conf();
    if (config._id !== expectedSet || config.members.length !== 1 || config.members[0]._id !== 0 || config.members[0].host !== expectedHost) {
      throw new Error("existing replica-set topology does not match the required single-node configuration");
    }
  ' >/dev/null

run_mongo '
    const databaseName = process.env.MONGO_DATABASE;
    const username = process.env.MONGO_APP_USERNAME;
    const password = fs.readFileSync("/run/secrets/mongo_app_password", "utf8").trim();
    if (!password) throw new Error("Mongo application secret is empty");
    const applicationDatabase = db.getSiblingDB(databaseName);
    const roles = [{ role: "readWrite", db: databaseName }];
    if (applicationDatabase.getUser(username) === null) {
      applicationDatabase.createUser({ user: username, pwd: password, roles });
    } else {
      applicationDatabase.updateUser(username, { pwd: password, roles });
    }
  ' >/dev/null

printf 'Mongo replica set and least-privilege application user are ready.\n'
