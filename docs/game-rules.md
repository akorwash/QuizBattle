# QuizBattle MVP game rules

This document is the implemented MVP contract. Longer-term boundaries and aggregate diagrams are in [mvp-domain-design.md](mvp-domain-design.md).

## Match format

- Human arenas support 1v1, 2v2, 4v4, and open rosters from two to eight players. A bot arena is a private 1v1 contest between one authenticated human and one system-owned bot.
- Each human owns a starter collection of ten cards and commits five distinct, available cards. A bot receives five virtual question cards inside the match aggregate; those cards are not economy assets and can never enter the market or a trade.
- A committed card is locked to that match and cannot enter the market, a trade, or another match.
- The normal stage contains `5 × participant count` turns: one turn for each participant's card in every round.
- Every eligible participant may answer each question once.
- A turn lasts 20 seconds on the server clock and has a 3-second result reveal.
- Correct answer: 100 points plus `floor(50 × remaining milliseconds / 20,000)`.
- Wrong or missing answer: zero points.
- Duel/open arenas use the highest individual score. Team arenas first compare combined team scores and then select the highest-scoring player in the winning team as champion.
- Tied leaders enter repeated neutral-question tie-break rounds while question-bank capacity remains available; the engine never fabricates a winner.

### Bot behavior

- `random` chooses uniformly from the four options and waits a deterministic two-to-nineteen seconds.
- `smart` uses difficulty-aware accuracy (85% easy, 70% medium, 55% hard) and waits a deterministic two-to-fourteen seconds.
- The server derives each decision from a private per-match seed. The persisted plan is stable across refreshes and retries, but the browser never receives the seed or controls the bot actor.
- Bot automation runs before human commands and timeout advancement, so disconnecting or delaying a refresh cannot erase an answer that was due before the deadline.

## Rewards and fair play

- PvP champion: 120 coins and one newly minted card.
- Another member of the winning PvP team: 90 coins and one newly minted card.
- PvP loser: 45 coins. Legacy resolved draws retain the existing 75-coin rule.
- Only the first ten reward-eligible PvP results per user per UTC day grant coins or a winner card. Later PvP matches remain playable but return a durable `capped` receipt.
- Human victory over `random`: 60 coins and one newly minted card.
- Human victory over `smart`: 100 coins and one newly minted card.
- Only the first three bot victories per user per UTC day grant rewards. Additional bot matches remain playable but return a durable `capped` receipt with no coins or card.
- Bot loss, unresolved draw, and forfeit: no reward. The bot never owns a wallet or collectible card.
- Forfeit during deck collection or active play: no coins for either player; all match locks are released.
- Card power, mastery and rarity are collectible progression signals and do not multiply match score.
- A winner card is minted from an active question using private deterministic selection, preferring a question the player does not own. It is never taken from an opponent. Existing cards change owner only through an explicit atomic market purchase or accepted trade.
- Wallet credit, card minting, the immutable ledger, the daily PvP/bot quota, the viewer-specific reward receipt, card unlocks, and the final settlement marker commit in one MongoDB transaction and are idempotent on retry.

This avoids pay-to-win scoring and prevents repeated forfeits or automated bot farming from creating unlimited currency.

The public development question bank contains its answer key and is therefore not cheat-resistant. High-value ranked rewards require a newly rotated server-private bank that is absent from the public repository and production image.

## Server-authoritative lifecycle

```text
lobby -> collecting_decks -> active -> completed
                           \-> forfeited
```

Browser messages are commands, never facts. The browser cannot submit scores, deadlines, ownership, correctness, or state transitions. Every mutation has an idempotency key and is persisted with optimistic version checks inside MongoDB transactions where ownership or money changes.

## Information disclosure

- A player sees only their own committed card IDs.
- The opponent's deck is represented only as ready/not ready.
- During an active turn, the client receives the prompt and four choices, but not the correct choice, explanation, or opponent answer.
- Correctness, answer details and explanation are revealed only after the turn resolves.
- There is no raw question-by-ID client route.

## Economy

- Starter grant: 600 coins and ten cards.
- Listing price: 10 to 100,000 coins.
- Market fee: 5%, with a minimum fee of one coin.
- A listing escrows its card for seven days.
- A direct trade can offer/request up to five cards and up to 100,000 coins on each side; it expires after 24 hours.
- Purchase and trade settlement update wallets, card owner/status, offer status, and append-only ledger entries atomically.
- Cancel, reject, expiry, match completion, and forfeit release the corresponding escrow.

## Still outside the MVP

- Private human invitations, spectators, matchmaking, ranks and seasons.
- Reconnect grace/automatic disconnect forfeit and an always-on match timer worker.
- Card crafting, packs, auctions, gifts, fraud scoring and moderation workflows.
- Multi-replica realtime distribution.

These features require a new rules revision and must preserve the current ownership, disclosure, idempotency, and ledger invariants.
