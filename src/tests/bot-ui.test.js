"use strict";

const assert = require("assert").strict;
const gameUI = require("../static/game.js");
const battleUI = require("../static/battle.js");

assert.equal(gameUI.normalizeMode("bot"), "bot");
assert.equal(gameUI.normalizeBotStrategy("random"), "random");
assert.equal(gameUI.normalizeBotStrategy("anything-else"), "smart");
assert.deepEqual(gameUI.creationPayload("bot", "random"), {
  mode: "bot",
  maxPlayers: 2,
  isPublic: false,
  opponentType: "bot",
  botStrategy: "random",
});
assert.deepEqual(gameUI.creationPayload("bot", "smart"), {
  mode: "bot",
  maxPlayers: 2,
  isPublic: false,
  opponentType: "bot",
  botStrategy: "smart",
});
assert.equal(gameUI.isBotGame({ mode: "bot", joinedUsers: [] }), true);
assert.equal(gameUI.isBotGame({ mode: "duel", opponentType: "bot", joinedUsers: [] }), true);
assert.equal(gameUI.isBotGame({ mode: "duel", joinedUsers: [{ id: "1" }, { id: "2", isBot: true }] }), true);
assert.equal(gameUI.isBotGame({ mode: "duel", opponentType: "human", joinedUsers: [] }), false);

assert.equal(battleUI.normalizeMode("bot"), "bot");
assert.deepEqual(battleUI.normalizeReward({
  status: "completed",
  rewardCoins: 999,
  reward: {
    status: "granted",
    coinsGranted: 60,
    card: { id: "91", category: "science", rarity: "rare" },
  },
}), {
  status: "granted",
  coinsGranted: 60,
  card: { id: "91", category: "science", rarity: "rare" },
  reason: "",
  legacy: false,
});
assert.deepEqual(battleUI.normalizeReward({ status: "completed", rewardsSettled: true, rewardCoins: 120 }), {
  status: "granted",
  coinsGranted: 120,
  card: null,
  reason: "",
  legacy: true,
});
assert.equal(battleUI.normalizeReward({ status: "forfeited", rewardsSettled: true }).status, "ineligible");
assert.equal(battleUI.normalizeReward({ status: "completed", rewardsSettled: false }).status, "pending");
assert.equal(battleUI.normalizeReward({ reward: { status: "capped", coinsGranted: -20, reason: "daily_cap" } }).coinsGranted, 0);
assert.equal(/الحد اليومي/.test(battleUI.rewardReasonLabel("daily_cap")), true);
assert.equal(/الحد اليومي/.test(battleUI.rewardReasonLabel("bot_daily_cap")), true);
assert.equal(/شروط المكافأة/.test(battleUI.rewardReasonLabel("unknown_code")), true);

console.log("PASS bot-ui");
