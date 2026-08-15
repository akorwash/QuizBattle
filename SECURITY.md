# Security policy

QuizBattle supports a single-replica production release through the reviewed Docker,
GitHub Actions, and Cloudflare delivery path documented in this repository.

## Historical credential incident (remediated 2026-08-15)

Earlier Git revisions contained three real credential classes: a Firebase service-account private key, a JWT signing secret, and a Coveralls repository token. Treat all three historical values as permanently compromised.

Remediation completed by the repository owner:

1. The historical Firebase service-account key was deleted and the Coveralls token was regenerated.
2. Production bootstrap generates a new deployment-specific JWT secret and the application rejects the historical secret fingerprint.
3. Secret-bearing objects were removed from every writable branch/ref and the rewritten branches were force-pushed in coordination with the owner.
4. A pinned Gitleaks full-history scan passed across all 206 reachable commits after rewriting.
5. GitHub Support was asked to purge immutable pull-request refs and cached legacy views. Those references contain only revoked credentials and must not be restored into a writable branch.

Anyone with an old clone must replace it with a fresh clone; do not merge or push the pre-rewrite history. Continue auditing historical cloud access logs according to the providers' retention windows.

## Reporting

Do not open a public issue for a suspected vulnerability or exposed credential. Contact the repository owner privately with the affected revision/file, reproduction steps, impact, and suggested mitigation. Never include live secrets, private keys, database contents, or personal data in a report.

## Current security model

- Browser authentication uses signed JWTs in `__Host-` HttpOnly, Secure, SameSite=Strict cookies outside development/test.
- Logout and account updates persist the current JTI revocation in MongoDB, maintain a bounded local cache, and close local active sockets immediately. WebSockets on another process periodically revalidate against the shared store, normally observing revocation within 15 seconds; store errors fail closed.
- Realtime rooms/events and rate-limit counters remain process-local. Run one replica until those components use shared infrastructure; session revocation itself survives restarts.
- Raw question IDs are not exposed. Questions are selected from committed cards and resolved by the authoritative match engine.
- Server identity controls ownership and membership; client-supplied user IDs are not trusted.
- Request bodies, chat messages, connections, lobby membership, and mutation rates are bounded.
- Production-like configuration requires secure cookies and certificate-verified MongoDB/Redis TLS.

## Secret handling

- Store secrets in the deployment platform, AWS Secrets Manager/SSM, or a local untracked `.env`.
- Never place a real secret in source, tests, examples, CI YAML, container layers, documentation, or command output.
- Use GitHub OIDC and short-lived cloud credentials.
- Rotate secrets on personnel/repository exposure and after any suspected disclosure.

## Deployment baseline

- Terminate HTTPS at a trusted proxy and configure only its exact CIDRs.
- Restrict direct network access to the application, MongoDB, and Redis.
- Run one application replica until realtime events and rate limits use shared infrastructure.
- Enable protected environments, least-privilege IAM, centralized logs, backups, monitoring, and alerting.
- Run tests, the race detector, `govulncheck`, `gosec`, CodeQL, a history secret scan, and a clean container build before release.
