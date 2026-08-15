# QuizBattle stabilization review — 2026-08-15

## Decision

The repository is now a substantially hardened, locally playable MVP. It remains **not production-ready**, but it now contains the two-player card-game engine, internal question bank, wallets, ownership ledger, market, and direct trades described in the later implementation addendum below.

Do not deploy until the historical credentials are revoked/rotated and removed from Git history. Ownership transfer and competitive rewards were implemented only after the rules and ledger invariants in `game-rules.md` were fixed.

## Scope reviewed

- Go configuration, authentication, controllers, services, repositories, MongoDB indexes, and shutdown behavior.
- Browser session handling, forms, DOM rendering, lobby behavior, battle page, and world chat.
- WebSocket authentication, membership, revocation, expiry, quotas, backpressure, and lifecycle races.
- Docker, Compose, GitHub Actions, dependency versions, README, security policy, and migration notes.
- Current Git tree and full Git history for known credential classes.

## Material fixes completed

| Area | Result |
| --- | --- |
| Identity and access | Server-derived identity, bounded HS256 JWTs, secure host cookies, durable JTI revocation, logout idempotency, IDOR removal, same-origin write checks |
| Questions | 1,573 validated internal Arabic questions; no raw question-by-ID client access; correct answers remain server-only until turn resolution |
| Battles | Owner/member authorization, public-only creation until invitations exist, atomic two-player cap, bounded lists, serialized single-process membership changes |
| Realtime | Authenticated sockets, origin checks, aggregate quotas, message limits, membership/session disconnects, expiry checks, backpressure, reconnect/reconcile behavior |
| Data | Cryptographic IDs, JSON string encoding for 64-bit IDs, unique/query/TTL indexes, batched user projection, legacy oversized-battle defenses |
| Browser | No token/identity local storage, text-only rendering of untrusted content, bounded chat DOM, safe POST form fallback, request timeouts, 401/session recovery |
| Configuration | Required non-placeholder secret, historical-secret fingerprint rejection, production TLS enforcement, trusted-proxy validation, bounded request concurrency |
| Supply chain | Current direct Go modules, pinned container bases, pinned Actions, one-build release flow, fail-closed immutable ECR tag, digest deployment |
| Documentation | Rebuilt README and security model; added rules, migration, and this review record |

## Verification evidence

The following checks passed on the final working tree:

- `go test -count=1 ./...`
- `go test -cover ./...`
- `go vet ./...`
- `go mod verify`
- `govulncheck v1.7.0`: zero reachable vulnerabilities
- `gosec v2.28.0`: zero findings
- `actionlint v1.7.12`: all GitHub Actions workflows valid
- Gitleaks v8.28.0: no findings in the current tree; three known credential findings remain in 206-commit history
- Node syntax checks for every file under `src/static`
- Question-bank validation: 1,573 valid questions, zero duplicate IDs/prompts, seven generator/validator tests
- Interactive browser checks at 320px, 390px, and desktop: RTL layout, no structural horizontal overflow, login, chat, collection, market/trades, and saved-result navigation
- Docker Compose configuration validation
- `git diff --check`

The repository now includes isolated real-Mongo transaction tests for listing/trade expiry, full-payload trade idempotency, settlement rollback when one of ten card locks is missing, and reward-free forfeit cleanup. Broader concurrent-command and driver-failure coverage is still required. The production image and MongoDB replica-set Compose environment were built and run locally, and the two-account E2E command completed chat, ten match turns, rewards, a market purchase, a direct trade, and a second forfeited match that preserved balances and released every card. The Linux race detector and clean image build remain enforced by CI.

## Implementation addendum

The subsequent MVP pass added:

- the `domain/match`, `domain/economy`, and `domain/question` modules;
- immutable per-turn question/card snapshots and viewer-safe match snapshots;
- transactional card locks, reward settlement, market purchases, direct trades, and append-only ledger entries;
- a validated 1,573-question JSONL bank and idempotent startup import;
- an Arabic RTL card system and responsive battle/economy interfaces;
- `cmd/e2e`, which refuses remote targets by default and exercises the complete two-account happy path plus forfeit recovery.

## Open deployment blockers

1. **Critical — historical credentials:** revoke the Firebase service-account key and Coveralls token, rotate any reused JWT secret, audit access, then rewrite every affected Git ref with repository-owner coordination.
2. **High — release provenance:** an existing ECR tag must fail closed or be accepted only after signature/attestation verification for the exact repository, workflow, commit, and digest.
3. **High — topology:** run one application replica. Realtime rooms/events and rate-limit counters are process-local even though session revocations are durable.
4. **High — lobby lifecycle:** add owner presence leases, abandoned-lobby cleanup, reconnect grace, and an always-on match finalizer instead of relying only on player snapshot traffic.
5. **Medium — evidence:** expand real-Mongo coverage to concurrent command races and injected driver failures; add committed browser automation and deployment smoke tests.
6. **Medium — operations:** add ingress/WAF limits, alerting, audit events, backups/restore drills, image scanning/SBOM/signing, task-definition policy validation, and explicit ECS autoscaling prevention.

## Next production-hardening order

The locally playable MVP is complete. Production work should proceed in this order:

1. Revoke and purge the historical credentials with repository-owner coordination.
2. Add owner leases, abandoned-room cleanup, reconnect recovery, and a background match finalizer.
3. Replace process-local realtime coordination and limits before enabling more than one application replica.
4. Add concurrent Mongo/WebSocket fault-injection tests and committed browser journeys.
5. Close release provenance, task-definition policy, image scanning, SBOM, and signing gaps.
6. Run an adversarial review before attaching paid or redeemable value to the virtual economy.
