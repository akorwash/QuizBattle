# QuizBattle

[Play QuizBattle](https://quizbattle.qubefyn.com)

QuizBattle is a competitive Arabic knowledge-card game inspired by **Saif Al-Ma'rifa**. Every collectible card is backed by an Arabic question, and each player brings five cards into human, team, open, or private bot arenas for up to eight participants.

> **Current status:** QuizBattle is a deployable, single-instance MVP with a hardened runtime baseline. It is delivered with Docker Compose and GitHub Actions and served behind Cloudflare. Accounts, the public lobby, persistent chat, the server-authoritative match engine, collectible cards, coins, the marketplace, and direct trades are implemented. See the production limitations below before scaling it.

## What works today

| Area | Current implementation |
| --- | --- |
| Accounts | Sign-up, sign-in, HttpOnly sessions, session revocation on logout, profile updates, date of birth, and player avatars |
| Player community | Public lobby and a MongoDB-backed world chat whose message identity is assigned by the server; the latest 50 messages return after refresh |
| Arena voice | Optional peer-to-peer WebRTC audio for 1v1 matches, with mute and leave controls and no server-side recording; voice is disabled in team/open arenas, which require an SFU before it can be implemented safely |
| Matches | 1v1, 2v2, 4v4, open arenas for 2–8 players, and private 1v1 bot challenges with random or smart strategies; 20-second questions and a 3-second answer reveal |
| Question bank | 1,573 sourced Arabic questions across nine categories, with content hashes and a standalone validator |
| Cards | Economy state is initialized on first use with ten starter cards; human winners receive a newly minted question card, while cards also include rarity, power metadata, play/win history, reviewable automatic deck selection, and transactional match locking |
| Coins | Economy state is initialized on first use with 600 coins; PvP rewards are 120 for the champion, 90 for each winning teammate, and 45 for each losing player; human wins in bot mode award 60 (random) or 100 (smart); both reward paths have daily anti-farming caps, and forfeits award nothing |
| Marketplace | List a card, buy atomically, cancel a listing, charge a 5% fee rounded down with a one-coin minimum, and prevent double sales |
| Direct trades | Card/coin-for-card/coin offers with accept, reject, and cancel; the sender's offered assets are escrowed transactionally, and requested assets are verified at acceptance |
| Mobile | Fully localized Arabic, right-to-left interface, fixed mobile navigation, and layouts starting at 320 px |
| Operations | Docker Compose, a MongoDB replica set, health/readiness endpoints, graceful shutdown, and database indexes |

Private human invitations, seasonal rankings, and spectator mode are not part of this MVP yet.

## MVP gameplay rules

- Fixed arena modes are `duel` (2 players), `team_2v2` (4 players), and `team_4v4` (8 players). The `open` mode accepts 2–8 players, according to the owner-selected capacity.
- A `bot` arena is a private two-seat duel. The human chooses a `random` bot for a lighter challenge or a `smart` bot whose accuracy varies by question difficulty. Bot timing and answers are planned from a private server seed, persist across refreshes, and cannot be submitted by the browser.
- In fixed modes, the owner selects **Prepare arena** after exactly 2, 4, or 8 players have joined. An open arena can be prepared with any roster from two players up to its configured capacity. Preparation freezes membership and prevents further joins.
- Each owner may run at most three concurrent arenas. Completed and forfeited arenas remain in history but do not count toward this limit.
- On the first collection or economy request, the server transactionally initializes the account with ten cards and 600 coins.
- Each player commits five owned, available cards. Committing the deck marks the player ready and locks those cards to the match.
- **Auto build** ranks cards by power, rarity, win rate, wins, and finally stable card ID. It only proposes a deck; the player can review, replace, or manually select cards before committing it.
- Only the arena owner can start the match, and the start action is unavailable until every player is ready.
- A normal match contains `5 × player count` turns. On each turn, every eligible player may submit at most one answer within the server-authoritative 20-second timer.
- A correct answer awards 100 points, with a speed bonus of up to 50 additional points.
- The correct answer and explanation are not sent to the browser until the turn closes.
- Card power and rarity are metadata used by deck selection; they do not multiply match points, so the game is not pay-to-win.
- In an open arena, the highest-scoring player wins. In 2v2 and 4v4, the team with the highest combined score wins, and its highest-scoring member becomes the final champion.
- If multiple leaders are tied, the engine starts neutral tie-break questions for those contenders only. It repeats while questions are available; if the tie-break pool is exhausted, the match waits for replenishment instead of declaring a false winner.
- A completed human victory mints one new card from the active question bank; it never takes a card from an opponent. Existing cards only change owners through the marketplace or an explicit direct trade.
- PvP keeps the 120/90/45 coin schedule and grants one new card to each human winner for the first ten reward-eligible PvP results per user per UTC day. A bot victory grants the human 60 coins against `random` or 100 against `smart`, plus one new card. Only the first three bot wins per user per UTC day are rewarded. Matches beyond either limit remain playable and are still resolved by points, but award nothing. A loss, unresolved draw, or forfeit against a bot awards nothing.
- A duel participant—or the arena owner in team/open modes—may forfeit. Forfeiting ends the match without rewards and releases every locked card.

The detailed domain contract is documented in [docs/mvp-domain-design.md](docs/mvp-domain-design.md), and the visual card system is documented in [docs/card-design-spec.md](docs/card-design-spec.md).

## Question bank

The canonical dataset is [data/question-bank/questions.ar.jsonl](data/question-bank/questions.ar.jsonl):

| Category | Questions |
| --- | ---: |
| Mathematics | 500 |
| Geography | 314 |
| Science | 236 |
| Cities | 157 |
| Religion | 114 |
| Technology | 86 |
| Civics and politics | 68 |
| General knowledge | 54 |
| History | 44 |
| **Total** | **1,573** |

Every record contains four unique options, the correct answer, an explanation, a source, a verification date, and a `contentHash`. The validator rejects duplicate IDs, duplicate prompts, and invalid record shapes. Primary data sources are listed in [data/question-bank/SOURCES.md](data/question-bank/SOURCES.md) and include Wikidata, IANA, BIPM, IUPAC, UN M49, and Quran Foundation. Religion questions are limited to Quran chapter names and ordering and should receive specialist human review before a broad public launch.

> **Competitive-integrity warning:** this public development bank includes its answer key and is already present in Git history. It is suitable for local/MVP testing, but not for valuable ranked rewards: a player can build an offline answer lookup. Before treating coins, cards, seasons, or rankings as high-value competitive outcomes, deploy a newly rotated server-private bank through `QUESTION_BANK_PATH`, keep it out of the repository and image, and retire the public questions from rewarded play.

Validate the bank with:

```bash
python tools/question-bank/validate.py --minimum 1000
python -m unittest discover -s tools/question-bank -p "test*.py"
```

## Architecture

```text
Browser (Arabic HTML/CSS/JavaScript)
  ├─ REST commands/queries ─> middleware ─> controllers ─> application services
  ├─ WebSocket events/chat/signaling ─> authenticated bounded registry/hubs
  ├─ World-chat history    ─> newest 100 Mongo messages with a 7-day TTL
  ├─ Arena audio           ─> WebRTC peer-to-peer (server relays signaling only)
  └─ Safe match snapshots  <─ domain aggregates ─> transactional Mongo repositories

Domain modules: avatar | chat | economy | match | question
Mongo collections: users, Game, Matches, QuestionBank, Cards, Wallets,
                   MarketListings, TradeOffers, EconomyLedger, SessionRevocation,
                   RewardQuotas, ChatMessage, UserAvatar
```

The application is a modular monolith written in Go. Transactions that change coin balances or card ownership require a MongoDB replica set. Realtime events and rate-limit state are still process-local, so run **one application instance only** until a shared broker and distributed state are implemented.

## Local development

### Docker Compose

1. Copy `.env.example` to `.env`.
2. Generate a unique random `JWT_SECRET` containing at least 32 characters. The example deliberately leaves it empty.
3. Start the stack:

```bash
docker compose up --build
```

Open `http://127.0.0.1:8080`. Compose creates a single-node MongoDB replica set and imports the question bank when `SEED_DATABASE=true`.

### Run Go directly

Requirements: Go 1.26.6 and a locally reachable MongoDB replica set. The application reads the process environment directly and does not load `.env` files. Before running from `src`, export the required local values. For example:

```bash
export APP_ENV=development
export MONGO_URI='mongodb://127.0.0.1:27017/?replicaSet=rs0'
export MONGO_DATABASE=quizbattle
export JWT_SECRET="$(openssl rand -hex 32)"
export SEED_DATABASE=true
go mod download
go run .
```

In development, `COOKIE_SECURE` defaults to `false`. Every non-development/test environment requires a Secure session cookie and certificate-verified MongoDB TLS.

## End-to-end testing

With the local stack running, the following command creates unique local test accounts and exercises persistent chat, authenticated WebRTC signaling without activating a microphone, a complete human 1v1 match, a complete smart-bot match with an exact coin/card reward check, preparation and startup of 2v2, 4v4, and open arenas, the marketplace, direct trades, and card release after a forfeit:

```bash
cd src
go run ./cmd/e2e -base http://127.0.0.1:8080
```

The E2E command rejects non-loopback hosts by default so it cannot accidentally create test data in a remote environment.

## Configuration

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `MONGO_URI` | Yes | — | MongoDB URI; certificate-verified TLS is required outside development/test |
| `MONGO_DATABASE` | Yes | — | MongoDB database name |
| `JWT_SECRET` | Yes | — | Deployment-specific random secret containing at least 32 characters |
| `APP_ENV` | No | `production` | Use `development` or `test` only for local work |
| `RELEASE_SHA` | Outside development/test | — | Full 40-character lowercase hexadecimal commit SHA exposed by health checks to prevent deploying the wrong release |
| `PORT` | No | `8080` | HTTP port |
| `SESSION_TTL` | No | `1h` | Session lifetime, from 15 minutes to 24 hours |
| `COOKIE_SECURE` | No | Environment-dependent | Cannot be disabled outside development/test |
| `ALLOWED_ORIGINS` | No | Same origin | Additional origins accepted by write/WebSocket origin checks; does not enable CORS |
| `TRUSTED_PROXY_CIDRS` | No | Empty | CIDRs of actual trusted load balancers only |
| `SEED_DATABASE` | No | `false` | Idempotently import the question bank at startup |
| `QUESTION_BANK_PATH` | No | `../data/question-bank/questions.ar.jsonl` | Path to the internal JSONL question bank |
| `REDIS_ADDRESS` | No | Empty | Optional Redis endpoint for future distributed events |
| `REDIS_USERNAME` | No | Empty | Redis ACL username |
| `REDIS_PASSWORD` | No | Empty | Redis password |
| `REDIS_TLS` | No | `false` | Required when Redis is used outside development/test |

The application does not support the legacy `ACCESS_SECRET` fallback. Never commit `.env`, cloud credentials, JWT secrets, or CI tokens.

## HTTP and WebSocket interfaces

Public:

- `GET /`, `/about`, `/contact`, `/auth/signin`, `/auth/signup`
- `POST /user/createuser`, `POST /user/login`, `POST /user/logout`
- `GET /healthz` — returns `{"status":"ok","release":"<commit-sha>"}` in production; `release` may be empty in development/test
- `GET /readyz` — available on the application listener for local/orchestrator checks; production Nginx intentionally returns 404

Authenticated account and lobby:

- `GET /user/profile`, `GET /game/play`, `GET /battle/{id}`
- `GET /user/profile/{username}` — permanent redirect to the canonical profile page
- `GET /api/v1/session`, `POST /api/v1/user`
- `PUT /api/v1/user/avatar`, `DELETE /api/v1/user/avatar` — upload one JPEG/PNG `avatar` field up to 2 MiB, re-encoded as a metadata-free 512×512 JPEG
- `GET /api/v1/user/avatar/{id}` — any authenticated user may fetch another player's processed avatar; responses use `private, no-cache`
- `GET /api/v1/chat/messages` — returns the latest 50 stored messages in ascending order
- `POST /api/v1/game`
- `POST /api/v1/game/{id}/join`, `POST /api/v1/game/{id}/exit`
- `GET /api/v1/game/{id}`, `GET /api/v1/games/public`, `GET /api/v1/games/mine`

Authenticated match:

- `POST /api/v1/game/{id}/prepare` — the owner freezes membership and opens deck preparation
- `PUT /api/v1/game/{id}/deck`
- `POST /api/v1/game/{id}/start`
- `GET /api/v1/game/{id}/match`
- `POST /api/v1/game/{id}/answer`
- `POST /api/v1/game/{id}/forfeit`

Authenticated economy:

- `GET /api/v1/collection`
- `GET /api/v1/market`, `POST /api/v1/market/listings`
- `POST /api/v1/market/listings/{id}/buy`, `POST /api/v1/market/listings/{id}/cancel`
- `GET /api/v1/trades`, `POST /api/v1/trades`
- `POST /api/v1/trades/{id}/accept`, `POST /api/v1/trades/{id}/reject`, `POST /api/v1/trades/{id}/cancel`

Realtime:

- `GET /ws/events`
- `GET /ws/world-chat`
- `GET /ws/game/{id}`

A world-chat message is persisted before it is broadcast. The server keeps the newest 100 messages with a seven-day MongoDB TTL; TTL deletion is eventual. The arena channel accepts only the supported WebRTC signaling events (`voice_ready`, `voice_leave`, `voice_offer`, `voice_answer`, and `voice_ice`) and overwrites the sender identity with the authenticated session identity. Audio never passes through the application server and is not recorded. Microphone access is requested only after the player explicitly joins voice chat.

There is no raw-question endpoint. Normal-round questions come from committed cards, while tie-break questions are selected neutrally from the bank. The server returns player-specific safe snapshots, and public 64-bit identifiers are serialized as JSON strings to preserve JavaScript precision.

## Quality and security

From `src`:

```bash
go fmt ./...
go vet ./...
go test -count=1 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
go run github.com/securego/gosec/v2/cmd/gosec@v2.28.0 -quiet ./...
```

Review [SECURITY.md](SECURITY.md), the [2026 security review](docs/review-report-2026-08-15.md), and the [migration notes](docs/migration-notes.md) before changing production infrastructure.

## Production deployment

The production service is available at [quizbattle.qubefyn.com](https://quizbattle.qubefyn.com). The production deployment and recovery procedure is documented in [docs/production-deployment.md](docs/production-deployment.md).

Current production constraints:

- Historical Firebase and Coveralls credentials have been revoked. Production's JWT secret is generated once during the one-time secret bootstrap and is reused across deployments. Rewritten branch/tag history passes full-history secret scanning; removing old pull-request refs and cached objects through GitHub Support remains a follow-up cleanup step.
- Run one application instance only. Do not enable autoscaling or zero-downtime overlap until WebSocket state, events, membership revocation, and rate limits use shared infrastructure.
- Runtime secrets are stored in root-only server files. MongoDB is internal, TLS-authenticated, and backed up in encrypted form. Copy encrypted backups and the recovery key off-host and test restoration regularly.
- The production pipeline runs tests, the race detector, `govulncheck`, `gosec`, and full-history secret scanning. It builds an immutable image by digest, emits an SBOM, and validates the production topology before deployment.
- Configure TURN with short-lived server-issued credentials before considering arena voice reliable on all networks; STUN alone cannot traverse every restricted NAT.
- The bundled public question bank exposes its answer key. Production rewards must remain virtual/low-stakes until a newly rotated server-private bank replaces it.

## Contributing

The `master` branch is protected from direct pushes, force pushes, and deletion. Create a feature branch, open a pull request, resolve review conversations, keep the branch up to date, and wait for both required GitHub Actions checks to pass before merging:

- `Test and validate release inputs`
- `Test, vet, and security checks`

## License

QuizBattle is released under the [MIT License](LICENSE).
