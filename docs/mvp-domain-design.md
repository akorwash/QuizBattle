# QuizBattle playable MVP domain design

Status: implementation contract for the first locally playable release.

## Scope and assumptions

The MVP is a modular Go monolith backed by MongoDB. It supports two to eight
human players across duel, 2v2, 4v4 and open arenas, plus a private one-human/one-bot duel, an internal Arabic question bank, collectible question cards, a
wallet, a marketplace, direct card trades, a social lobby, and a complete
server-authoritative match. Horizontal scaling, ranked matchmaking,
spectators, real-money purchases, and battle-forced card transfer are
outside this release.

Full DDD is justified for Match and Economy because rules change quickly and
currency/card ownership require explicit, auditable invariants. Account and
QuestionBank remain simpler supporting contexts.

## Strategic model

| Context | Kind | Responsibility |
| --- | --- | --- |
| Match | Core | Lobby readiness, committed decks, timed turns, answers, score and final result |
| Collection & Economy | Core | Card ownership, escrow, wallet balances and immutable ledger entries |
| Bot policy | Supporting | Private server-seeded decisions and virtual decks for random/smart PvE duels |
| Marketplace & Trade | Supporting | Listings, purchases, bilateral offers, expiry and settlement |
| QuestionBank | Supporting | Curated/versioned questions and safe delivery to active turns |
| Social | Generic | World chat and lobby presence; never authoritative for match state |
| Identity | Generic | Existing account, session and server-derived user identity |

All contexts live in one process for now but communicate through narrow
service/repository contracts. Browser messages are commands, never facts.

## Ubiquitous language

- **Question**: immutable, reviewed knowledge content with four options and one
  server-only correct option.
- **Card**: a tradable instance that references one question and has exactly one
  owner outside escrow.
- **Deck commitment**: five distinct cards locked for one match.
- **Prepared roster**: the owner-frozen membership after a lobby reaches the mode requirement.
- **Ready player**: a prepared participant with five valid committed cards.
- **Bot participant**: a match-scoped system actor with a private decision seed and virtual deck; never an account, wallet, or tradable card owner.
- **Turn**: one timed question from one committed card. Every eligible player may answer once.
- **Main stage**: five cards per participant, producing `5 × player_count` turns.
- **Tie-break**: repeated neutral questions restricted to tied leaders until one champion remains.
- **Wallet**: a non-negative integer coin balance.
- **Ledger entry**: immutable record of every coin change or card transfer.
- **Listing**: one card escrowed at a fixed coin price.
- **Trade offer**: escrowed cards/coins proposed by one player to another.
- **Settlement**: one atomic transaction that changes ownership/balances and
  appends the matching ledger records.
- **Reward receipt**: a durable viewer-specific statement of granted, capped, or ineligible rewards.

## Match rules

- Duel requires 2 players, 2v2 requires 4, 4v4 requires 8, and open accepts 2–8 up to its configured capacity.
- Bot mode is a private duel with one human and one server-owned participant. `random` answers uniformly; `smart` uses difficulty-aware accuracy. Both use deterministic private-seed timing so retries and refreshes cannot change a planned decision.
- The owner prepares the arena to freeze its roster, and alone may start after every player is ready.
- Each player commits five distinct cards they currently own and that are not
  listed, offered, or locked in another match.
- The server creates `5 × player_count` main turns by cycling the frozen roster for each card index.
- Each turn lasts 20 seconds. Each eligible player gets one answer. Missing the deadline
  is a no-answer.
- Correct answer score is `100 + floor(50 * remaining_ms / 20000)`; incorrect or
  missing answers score zero. Server receipt time is authoritative.
- A resolved turn is shown for three seconds before the next turn opens.
- Duel/open use the highest individual score. Team modes first use the sum of each team's scores, then select the highest individual inside the winning team as champion.
- Tied leaders enter repeated neutral-question sudden death. A team tie is resolved first; a champion tie inside the winning team is resolved second.
- In PvP, the champion earns 120 coins, another winning teammate earns 90, and every loser earns 45. Each human winner also receives one newly minted active-question card. Only the first ten reward-eligible PvP results per user per UTC day mint coins/cards.
- A human bot victory earns 60 coins against `random` or 100 against `smart`, plus one newly minted card. Only the first three bot wins per user per UTC day are rewarded; bot losses, draws, and forfeits award nothing.
- Rewards are idempotent and issued once at final settlement. The bot is excluded from wallets, collectible cards, ledger rewards, and human card-lock counts.
- Cards gain one play and mastery progress, but rarity/power never changes
  answer score in the MVP.
- Battle results never transfer card ownership. Ownership changes only through
  an explicit marketplace purchase or accepted trade.

## Aggregate invariants

### Match aggregate

- State moves only `collecting_decks -> active -> tie_break* -> completed|forfeited`.
- Between two and eight immutable prepared participants belong to a match. Human IDs are positive and unique; bot mode has exactly one negative, match-scoped system actor and one positive owner ID.
- A deck contains exactly five unique owned cards.
- A card appears in at most one committed deck and one active match.
- Only an active turn accepts answers; an eligible player answers it at most once.
- A fixed team roster is balanced and full. An open roster remains between its frozen minimum and maximum.
- Completion requires one final `winnerId`; pool exhaustion pauses at `tie_break/awaitingQuestion` for a server refill and never fabricates a draw.
- A command idempotency key takes effect at most once.
- Correct options and opponent answers are never exposed before resolution.
- Server time controls deadlines. Completed turns/matches cannot reopen.
- Bot answers due before a deadline are applied before timeout catch-up, even when no browser was connected at the due time.
- Optimistic `version` compare-and-swap prevents lost concurrent commands.

### Economy aggregate/transaction

- Wallet balance is always an integer greater than or equal to zero.
- A card has exactly one owner and one status: `available`, `match_locked`,
  `market_escrow`, or `trade_escrow`.
- Escrow has one matching active listing/offer/match and an expiry where
  applicable.
- A seller cannot buy their own listing; buyer and seller are server-derived.
- Prices are between 10 and 100,000 coins. Market settlement takes a 5% fee
  (minimum one coin) as a currency sink.
- One idempotency key maps to one command/result.
- Wallet/card mutations and ledger entries commit in the same MongoDB
  transaction. Local MongoDB therefore runs as a single-node replica set.
- Match reward coin credit, card mint, quota reservation, card release, receipt, and settlement marker commit in one transaction. A retry observes the same result rather than minting twice.

## Persistence

| Collection | Important keys/indexes |
| --- | --- |
| `QuestionBank` | unique `id`; `status,category,difficulty`; `contentHash` |
| `Cards` | unique `id`; `ownerId,status`; `questionId`; editions are assigned inside the same serialized reward transaction |
| `Wallets` | unique `userId`; non-negative balance validation in service/update |
| `EconomyLedger` | unique `id`; unique `idempotencyKey,entryPart`; `userId,createdAt` |
| `RewardQuotas` | deterministic user/day key; `userId,day`; TTL cleanup after the anti-farming window |
| `MarketListings` | unique `id`; unique active `cardId`; `status,createdAt`; expiry |
| `TradeOffers` | unique `id`; `senderId,status`; `receiverId,status`; expiry |
| `Matches` | unique `id`; unique `gameId`; `playerIds,status`; `version` |
| existing `Game` | `mode`, `maxPlayers`, `state`, `matchId`, frozen membership and created timestamp |

Question content is versioned by hash. Existing matches embed the selected
prompt/options/correct index so a later editorial correction cannot change an
active or historical result.

## HTTP contract

All endpoints require the existing secure session and same-origin checks.

- `GET /api/v1/collection` - wallet summary and owned card metadata; no answer.
- `GET /api/v1/collection` also idempotently ensures starter wallet/cards for a new account.
- `GET /api/v1/market` - active listings with cursor/limit.
- `POST /api/v1/market/listings` - list one owned available card.
- `POST /api/v1/market/listings/{id}/buy` - atomic purchase.
- `POST /api/v1/market/listings/{id}/cancel` - seller cancellation.
- `GET|POST /api/v1/trades` - list/create direct offers.
- `POST /api/v1/trades/{id}/accept|reject|cancel` - settle or release escrow.
- `POST /api/v1/game/{id}/prepare` - owner freezes a valid roster and creates the match draft.
- `PUT /api/v1/game/{id}/deck` - commit five owned cards.
- `POST /api/v1/game/{id}/start` - owner starts only after every frozen player commits.
- `GET /api/v1/game/{id}/match` - sanitized snapshot and lazy timeout advance.
- `POST /api/v1/game/{id}/answer` - `{turnId, optionId, commandId}`.

Every write accepts an explicit command/idempotency ID generated by the client;
the server binds it to the authenticated user and action.

## Realtime contract

The battle WebSocket remains server-to-client. Gameplay commands use bounded
HTTP requests so authorization, idempotency, errors, and retries stay explicit.
Server events are small invalidation notices:

- `match_created`, `deck_committed`, `match_started`, `turn_resolved`,
  `turn_started`, `tiebreak_started`, `match_completed`, `player_left`.

Each event includes `gameId` and, for match changes, `matchVersion`; clients fetch the
authoritative snapshot. World chat is social-only and cannot alter economy or
match state.

## Starter economy and card rarity

- First collection access atomically creates a 600-coin wallet and ten distinct
  starter cards spanning at least five categories.
- Rarity derives from reviewed question difficulty: easy/common,
  medium/rare, hard/epic. Legendary is reserved for later authored seasonal
  cards and is not randomly fabricated.
- Coins enter through one starter grant and match rewards. Coins leave through
  the 5% market fee. No negative balance, client-supplied reward, or admin-free
  mint endpoint exists.
- Winner cards are selected from active reviewed questions with private deterministic ranking. Selection prefers questions not already owned; duplicates become later editions only after the active pool has been collected.

## Multiplayer acceptance scenario

1. Register Alice and Bob; first collection request gives each 600 coins and ten cards.
2. Alice creates a mode-specific public lobby; the server enforces its 2/4/8/open capacity atomically.
3. Alice prepares a valid roster; later joins fail. Every participant commits five owned cards.
4. Start remains blocked until everyone is ready, then Alice starts. The server creates `5 × player_count` turns.
5. The final snapshot is identical after retries and rewards are written once.
6. A tied top score opens restricted neutral-question rounds until one champion remains.
7. Alice lists an unlocked card for 100 coins; Bob buys it. Ownership moves,
   Bob pays 100, Alice receives 95, and the ledger records the 5-coin fee.
8. Bob offers one available card for one of Alice's; Alice accepts and the two
   ownership changes commit atomically.
9. Participants exchange world-chat messages and can reconnect on mobile without losing
   the authoritative match/economy state.
10. Alice creates a private smart-bot arena. The server creates the virtual bot deck, applies delayed bot answers during lazy catch-up, and—if Alice wins—atomically grants 100 coins and one new card. A repeated settlement returns the same receipt.

## Rollout and evidence

1. Import and validate the versioned question bank.
2. Ship collection/wallet and starter grants behind the authenticated API.
3. Ship pure Match aggregate tests, then Mongo CAS integration tests.
4. Enable local replica-set transactions and economy integration tests.
5. Build the mobile-first battle, collection, market, trade, and social views.
6. Run two isolated browser contexts through the full acceptance scenario.
7. Repeat security/static/dependency scans before considering deployment.

Open risks remain email/mobile verification, historical leaked credentials,
single-process realtime/rate limits, broader abuse/fraud controls beyond the PvP/bot daily caps, a public development answer key that must be replaced by a rotated server-private bank before high-value ranked play, and the absence of a
production payment/compliance model. Coins are strictly virtual and cannot be
purchased or redeemed in this MVP.
