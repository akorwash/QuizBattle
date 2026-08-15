"use strict";

const assert = require("assert").strict;
const fs = require("fs");
const path = require("path");

function readView(name) {
  return fs.readFileSync(path.join(__dirname, "..", "api", "view", name), "utf8");
}

const arenaContracts = [
  'id="gameModeBot"',
  'name="gameMode" value="bot"',
  'aria-controls="botStrategyControl"',
  'id="botStrategyControl"',
  'id="botStrategyRandom"',
  'name="botStrategy" value="random"',
  'id="botStrategySmart"',
  'name="botStrategy" value="smart"',
  'id="arenaPrivacyHint"',
  'id="creategame"',
];

["gameplay.html", "user.html"].forEach(function (name) {
  const page = readView(name);
  arenaContracts.forEach(function (contract) {
    assert.equal(page.includes(contract), true, name + " missing " + contract);
  });
  const ids = [];
  page.replace(/\sid="([^"]+)"/g, function (_, id) { ids.push(id); return _; });
  const duplicates = ids.filter(function (id, index) { return ids.indexOf(id) !== index; });
  assert.deepEqual(duplicates, [], name + " contains duplicate DOM ids");
});

const battle = readView("battle.html");
[
  'id="botBattleBadge"',
  'id="botActivity"',
  'id="voiceChatPanel"',
  'id="matchResult"',
  'tabindex="-1"',
  'id="rewardReceipt"',
  'id="rewardCoinsGranted"',
  'id="rewardCard"',
  'id="rewardCardArt"',
  'href="/user/profile#collection"',
  'href="/game/play"',
].forEach(function (contract) {
  assert.equal(battle.includes(contract), true, "battle.html missing " + contract);
});

console.log("PASS bot-dom");
