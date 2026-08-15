# Security policy

QuizBattle is a locally playable pre-release MVP with no supported production release.

## Known pre-release blocker

Earlier Git revisions contained three real credential classes: a Firebase service-account private key, a JWT signing secret, and a Coveralls repository token. They are absent from the current working tree, but deletion does not make a leaked credential safe.

Before any deployment or public mirror:

1. Revoke the Firebase key and audit the related cloud IAM permissions and access logs.
2. Revoke the Coveralls token.
3. Rotate any JWT secret that was used or reused and invalidate all sessions signed by it.
4. Purge the secret-bearing objects from every Git ref and coordinate the required force-push/clone replacement.
5. Run a full-history secret scan again.

History rewriting is intentionally not performed automatically because it is destructive and requires repository-owner coordination.

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
