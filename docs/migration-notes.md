# Migration notes for the hardened prototype

Review and back up the database before running the updated service against data created by older revisions.

## Collection compatibility

MongoDB collection names are case-sensitive. The application intentionally keeps the legacy names `users`, `Question`, and `Game`. The MVP adds `QuestionBank`, `Matches`, `Cards`, `Wallets`, `MarketListings`, `TradeOffers`, `EconomyLedger`, `RewardQuotas`, and `SessionRevocation`.

Card locks, wallet settlement, market purchases, and trades use multi-document transactions. MongoDB must therefore run as a replica set even for local development; Compose initializes the single-node `rs0` set automatically.

## Unique-index preflight

Startup now creates unique indexes for user/game/question IDs and for normalized username, email, and mobile fields. Before the first upgraded start, audit duplicates and malformed/missing IDs in a backup copy. Also identify legacy usernames/emails containing uppercase or surrounding whitespace before normalizing them; normalization can turn two previously distinct values into one.

Index creation intentionally fails instead of guessing which account or record to delete. Resolve collisions with an owner-approved migration and retain an audit record.

## Legacy battle membership

Older code allowed an unbounded `joinedusers` array. The current multiplayer MVP atomically caps new joins at the arena's validated capacity, with a hard maximum of eight, and excludes malformed oversized battles from lists. Direct access to such a battle is rejected.

Before deployment, identify active `Game` documents with more than eight joined users—or more members than their persisted mode/capacity allows—preserve an audit copy, and close or repair them according to a product-approved policy. The application does not silently truncate player membership because that would discard user state.

Bot arenas add a separate `bot` seat to `Game`; they do not insert the negative bot actor into `joinedusers` or create a user document. Matches created before `rewards-v1` intentionally retain their original coin-only settlement behavior. New matches persist a private reward nonce and viewer-specific receipts; never backfill a nonce into an old terminal match because doing so could mint retroactive cards.

`rewards-v1` stores UTC-day quota documents for both bot victories (three) and reward-eligible PvP results (ten). The limits apply only to newly stamped matches. The bundled public question bank contains an already disclosed answer key; migrate rewarded production play to a newly rotated server-private bank rather than relying on obscured question IDs.

## Question-bank import

`SEED_DATABASE=true` imports the validated JSONL bank idempotently from `QUESTION_BANK_PATH`. Back up `QuestionBank` before replacing the source file. Never delete a question referenced by an existing card or match snapshot without a retirement migration; historical match snapshots retain their question text/hash for auditability.

## Economy preflight

Starter wallets/cards are created lazily and idempotently. Before enabling the economy on legacy accounts, audit for cards with unknown owners, non-empty `lockRef` values that have no matching active match/listing/trade, negative wallet balances, or `locked > balance`. Do not repair ownership or balances without an append-only audit entry.

## Battle ordering

New games receive a `createdat` timestamp and public lists sort by it. Legacy game documents without this field remain readable but sort behind timestamped games. Backfill only from a trustworthy historical source; random IDs do not encode creation time.

## Session and credential cutover

- Rotating `JWT_SECRET` invalidates every existing session.
- Secure deployments change the cookie name to `__Host-quizbattle_session`; old non-secure development cookies are not accepted there.
- Session revocations are stored in MongoDB with a TTL index and survive application restarts. Rotate any historically exposed JWT secret anyway, because revoking individual unknown tokens cannot repair a leaked signing key.

## Deployment topology

Use one application replica. MongoDB-backed session revocations are shared, but multiple replicas still split WebSocket rooms/events and rate-limit counters. Horizontal scaling requires shared event delivery and distributed limits first.
