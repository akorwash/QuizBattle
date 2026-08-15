# QuizBattle MVP game rules

This document is the implemented MVP contract. Longer-term boundaries and aggregate diagrams are in [mvp-domain-design.md](mvp-domain-design.md).

## Match format

- Exactly two authenticated players.
- Each player owns a starter collection of ten cards and commits five distinct, available cards.
- A committed card is locked to that match and cannot enter the market, a trade, or another match.
- The match contains five rounds and ten alternating turns: one turn for each player's card in every round.
- Both players may answer each question once.
- A turn lasts 20 seconds on the server clock and has a 3-second result reveal.
- Correct answer: 100 points plus `floor(50 × remaining milliseconds / 20,000)`.
- Wrong or missing answer: zero points.
- Highest total wins; equal totals are a draw.

## Rewards and fair play

- Winner: 120 coins.
- Loser: 45 coins.
- Draw: 75 coins per player.
- Forfeit during deck collection or active play: no coins for either player; all match locks are released.
- Card power, mastery and rarity are collectible progression signals and do not multiply match score.
- Match results never force ownership transfer. A card changes owner only through an explicit atomic market purchase or accepted trade.

This avoids pay-to-win scoring and prevents two colluding users from minting coins by repeatedly forfeiting.

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

- Private invitations, teams, spectators, bots, matchmaking, ranks and seasons.
- Reconnect grace/automatic disconnect forfeit and an always-on match timer worker.
- Card crafting, packs, auctions, gifts, fraud scoring and moderation workflows.
- Multi-replica realtime distribution.

These features require a new rules revision and must preserve the current ownership, disclosure, idempotency, and ledger invariants.
