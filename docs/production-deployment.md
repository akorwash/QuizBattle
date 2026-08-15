# QuizBattle production deployment

QuizBattle is deployed as an isolated Docker Compose project on the existing
Qubefyn host. Application releases are immutable GHCR digests; the server never
builds from a checkout and the deployment path never runs `docker compose down`.

## Security gate status

The historical credential gate in [`SECURITY.md`](../SECURITY.md) was completed on
2026-08-15: the Firebase key and Coveralls token were revoked/replaced, production
uses a newly generated JWT secret, writable refs were rewritten, and a pinned
full-history scan passed across all 206 reachable commits. GitHub Support cleanup
of immutable pull-request refs and cached legacy views remains a follow-up; those
views contain revoked values only and must never be merged back into a writable
branch.

The repository is public, so continue treating every credential ever committed to
it as compromised. Production bootstrap always generates a new deployment-specific
JWT secret and never reuses `ACCESS_SECRET`. Every production run repeats the full
history scan before it is allowed to publish an image.

## Production topology

- Host: `95.216.214.178`
- Public origin: `https://quizbattle.qubefyn.com`
- Deployment directory: `/home/Qubefyn/QuizBattle`
- Compose project: `quizbattle`
- Application bind: `127.0.0.1:3200`
- Application container port: `8080`
- Container network: `172.30.8.0/24`, gateway `172.30.8.1`
- Image: `ghcr.io/akorwash/quizbattle@sha256:<digest>`
- Durable data: the production MongoDB named volume; it is never removed by the
  deployment script.

Only Nginx listens publicly. MongoDB is not published on a host port. The app
trusts forwarded client addresses only from the fixed Docker gateway, so production
must retain `TRUSTED_PROXY_CIDRS=172.30.8.1/32`. `ALLOWED_ORIGINS` must contain only
`https://quizbattle.qubefyn.com`.

The deployment intentionally runs one application replica. Recreating it may
briefly disconnect active WebSockets; the browser reconnects to the new release.
Running two replicas requires distributed WebSocket/game coordination and must not
be enabled merely to hide that short release interruption.

## Release pipeline

[`.github/workflows/production.yml`](../.github/workflows/production.yml) runs on
pushes and pull requests targeting `master`.

1. The quality job verifies Go formatting/modules, runs `go vet` and race-enabled
   tests, runs browser-module and question-bank tests, builds the container, and
   validates the production Compose and shell files.
2. A push to `master` publishes `sha-<commit>` and `latest` discovery tags to GHCR.
   Deployment identity comes only from the registry's returned `sha256:` digest.
3. The exact digest is pulled and smoke-tested with a temporary MongoDB replica set.
   `/healthz` must report the commit SHA and `/readyz` must confirm MongoDB.
4. The production job checks the pinned SSH host key and the aggregate digest of
   every manually installed runtime file before installing temporary GHCR
   credentials and invoking the restricted deploy command.
5. The server verifies the RepoDigest and OCI revision label, recreates the Compose
   services without removing the project, checks local liveness/readiness and the
   public Cloudflare route, then records `.last-success`.
6. Failure restores the previous `.env` and image. On a failed first release, only
   the app is stopped; MongoDB and its volume remain available.

The rollback is an application rollback, not a database point-in-time restore.
Database/index changes must remain backward compatible with the prior image. Every
upgrade of an existing database first runs the root-owned encrypted `mongodump`
backup, and a systemd timer creates additional daily local snapshots. Copy encrypted
snapshots off the host and complete a documented restore drill before treating the
service as production data with a guaranteed RPO/RTO.

## GitHub configuration

Create a GitHub Environment named `production` and add one environment secret:

- `QUIZBATTLE_SSH_KEY`: the private half of a dedicated ED25519 deployment key.

The workflow's built-in `GITHUB_TOKEN` publishes and temporarily pulls the package.
Do not put `JWT_SECRET`, MongoDB credentials, Cloudflare API tokens, or the server
`.env` in GitHub build arguments. Runtime secrets remain in the root-owned
`/home/Qubefyn/QuizBattle/.env` file.

Generate a dedicated key locally:

```bash
ssh-keygen -t ed25519 -a 100 -f quizbattle_github_actions -C quizbattle-github-actions
```

Install `quizbattle_github_actions.pub` through the bootstrap procedure below and
store the complete private-key file as `QUIZBATTLE_SSH_KEY`. The bootstrap adds the
public key with `restrict` and a forced command. That key can only log in/out of
GHCR, request deployment/status, or calculate the reviewed runtime digest; it
cannot obtain an interactive root shell.

The verified server ED25519 host key is committed at
`.github/ssh/known_hosts`. Do not replace it with `ssh-keyscan` in CI. If the host
key legitimately changes, verify the new fingerprint out of band before updating
the repository.

## One-time server bootstrap

The host must already have Docker Engine with Compose v2, Nginx, Certbot, curl, and
the shared `/etc/nginx/conf.d/cloudflare-real-ip.conf` installed by Qubefyn.Site.
That include trusts `CF-Connecting-IP` only from Cloudflare's published networks.

Before obtaining the certificate, create this DNS record in the `qubefyn.com` zone:

| Type | Name | Value | Proxy during certificate issuance |
| --- | --- | --- | --- |
| A | `quizbattle` | `95.216.214.178` | DNS only |

Copy the reviewed `deploy/` directory and the public deployment key to the host,
then run the bootstrap as root from the copied repository layout:

```bash
bash deploy/bootstrap.sh \
  --deploy-public-key-file /root/quizbattle_github_actions.pub \
  --certbot-account CERTBOT_ACCOUNT_ID
```

Use `--certbot-account` when the host has more than one registered Let's Encrypt
account; it accepts exactly the 32-character lowercase hexadecimal account id and
validates it non-interactively before changing Nginx. Omit the option only when
`certbot show_account --non-interactive` can select the sole account. If no account
exists, rerun with a verified operations address via `--letsencrypt-email`. The
script never prints the account id or email.

The bootstrap performs these bounded actions:

- installs root-owned runtime files under `/home/Qubefyn/QuizBattle`;
- leaves `.env` and all credentials to the separate fail-closed secret bootstrap;
- adds the forced-command public key without changing other authorized keys;
- obtains a Let's Encrypt certificate through a temporary ACME-only HTTP vhost when
  needed;
- installs a root-owned Certbot deploy hook that validates and reloads Nginx after
  every successful renewal;
- backs up, validates, and atomically installs the QuizBattle Nginx vhost;
- installs the encrypted MongoDB backup service and timer definitions.

Confirm the renewal path after bootstrap (the deploy hook is deliberately included
in the dry run):

```bash
certbot renew --dry-run --run-deploy-hooks
```

Create the first-release sentinel, independent Mongo credentials, internal Mongo TLS
certificates, JWT secret, and backup recipient through the reviewed generator:

```bash
/home/Qubefyn/QuizBattle/bootstrap-secrets.sh \
  --bootstrap \
  --origin https://quizbattle.qubefyn.com
```

`--bootstrap` deliberately writes a zero image digest and `RELEASE_SHA=bootstrap`;
the app is not started. The first GitHub deployment replaces both atomically. Save
an offline copy of
`/home/Qubefyn/QuizBattle/secrets/backup-private-key.pem`; without that private key,
the encrypted database snapshots cannot be restored. Never commit or transmit the
generated `.env`, passwords, JWT secret, Mongo CA private key, server private key,
or backup private key through workflow logs.

Enable the daily backup schedule after `.env` exists:

```bash
systemctl enable --now quizbattle-mongo-backup.timer
systemctl status quizbattle-mongo-backup.timer --no-pager
```

The generated environment is root-owned mode `0600`. At minimum it contains these
non-secret deployment invariants:

```text
APP_ENV=production
APP_PORT=3200
RELEASE_SHA=bootstrap
ALLOWED_ORIGINS=https://quizbattle.qubefyn.com
TRUSTED_PROXY_CIDRS=172.30.8.1/32
COOKIE_SECURE=true
```

`bootstrap-secrets.sh` configures authenticated TLS MongoDB, a least-privilege app
user, and root-owned secret files; `production.env.example` is documentation only
and must not be copied as a live environment. Never make production start by
changing `APP_ENV` to `development` or by disabling secure cookies. Validate the
result without printing its values:

```bash
docker compose \
  --project-name quizbattle \
  --env-file /home/Qubefyn/QuizBattle/.env \
  -f /home/Qubefyn/QuizBattle/docker-compose.production.yml \
  config --quiet
```

Runtime contract changes are intentionally not copied by a normal application
deployment. Re-run `bootstrap.sh` from the exact reviewed checkout whenever the
production Compose, deploy/SSH/secret/init/backup scripts, backup systemd units,
Certbot renewal hook, or Nginx vhost changes. Until then, the workflow's
runtime-digest gate refuses deployment.

## Cloudflare cutover

After direct origin TLS has been issued and verified, enable the orange-cloud proxy
for `quizbattle.qubefyn.com`. **Full (strict) is required for this hostname before
the first production deployment**; ordinary Full is not sufficient because it does
not authenticate the origin certificate. The current zone-wide mode is Full, so add
a per-host Cloudflare rule for `quizbattle.qubefyn.com/*` that enforces Full (strict).
Do not change the zone-wide setting without checking every other hostname. If only
a zone-wide mode is available, first verify that every proxied origin has a valid
matching certificate, then change the zone safely.

Cloudflare Tunnel and the desktop WARP/Cloudflare One client are not part of this
deployment path. Traffic is Cloudflare proxy -> host Nginx -> loopback app.

Recommended Cloudflare controls:

- keep WebSockets enabled;
- bypass caching for `/api/*`, `/ws/*`, and `/healthz`;
- never cache responses carrying authentication cookies;
- allow `/static/*` to follow the origin's one-day cache policy;
- keep the origin firewall limited to required SSH/HTTP/HTTPS access, and consider
  restricting public web ports to Cloudflare networks only after certificate
  renewal behavior has been tested.

Verify the public path:

```bash
curl --fail --silent https://quizbattle.qubefyn.com/healthz
```

`/readyz` intentionally returns 404 through public Nginx and remains available only
on `http://127.0.0.1:3200/readyz` for deployment checks.

## Operations and rollback

The GitHub key exposes a read-only status command:

```bash
ssh -i quizbattle_github_actions root@95.216.214.178 status
```

Application logs remain available to an interactive server administrator:

```bash
docker compose \
  --project-name quizbattle \
  --env-file /home/Qubefyn/QuizBattle/.env \
  -f /home/Qubefyn/QuizBattle/docker-compose.production.yml \
  logs --tail 200 app mongo mongo-init
```

Deployments are serialized by the root-only
`/home/Qubefyn/QuizBattle/.deploy.lock`. An older GitHub
run is ignored after a newer run succeeds. `.env.previous` and `.last-success` are
root-only audit/rollback state. Do not delete the prior image until at least one
newer release has completed successfully.

The legacy `.github/workflows/aws.yml` is manual-only (`workflow_dispatch`) and does
not react to pushes or GitHub Releases. Hetzner production for this host is driven
only by `production.yml` on `master`; invoke the ECS workflow only for an explicitly
approved legacy recovery or migration exercise.
