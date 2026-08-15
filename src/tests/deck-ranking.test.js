"use strict";

const assert = require("assert").strict;
const ranking = require("../static/deck-ranking.js");

const cards = [
  { id: "90", status: "available", power: 9, rarity: "legendary", plays: 5, wins: 5 },
  { id: "10", status: "available", power: 10, rarity: "common", plays: 0, wins: 0 },
  { id: "30", status: "available", power: 9, rarity: "legendary", plays: 10, wins: 8 },
  { id: "40", status: "available", power: 9, rarity: "epic", plays: 10, wins: 10 },
  { id: "20", status: "match_locked", lockRef: "match:777", power: 9, rarity: "legendary", plays: 10, wins: 8 },
  { id: "05", status: "match_locked", lockRef: "match:999", power: 100, rarity: "legendary", plays: 1, wins: 1 },
  { id: "01", status: "market_escrow", power: 100, rarity: "legendary", plays: 1, wins: 1 },
];

assert.deepEqual(
  ranking.rankEligibleCards(cards, "777").map(function (card) { return card.id; }),
  ["10", "90", "20", "30", "40"],
  "ranking must prefer power, rarity, win rate, wins, then stable opaque id",
);
assert.deepEqual(
  ranking.strongestCards(cards, "777", 3).map(function (card) { return card.id; }),
  ["10", "90", "20"],
);
assert.equal(ranking.isEligible(cards[4], 777), true);
assert.equal(ranking.isEligible(cards[5], 777), false);
assert.equal(ranking.winRate({ plays: 0, wins: 2 }), 0);

console.log("PASS deck-ranking");
