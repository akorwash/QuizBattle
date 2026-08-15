(function (root, factory) {
  "use strict";

  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  if (root) root.QuizBattleDeckRanking = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  const RARITY_WEIGHT = Object.freeze({
    common: 1,
    rare: 2,
    epic: 3,
    legendary: 4,
  });

  function finiteNumber(value) {
    const number = Number(value);
    return Number.isFinite(number) ? number : 0;
  }

  function opaqueID(value) {
    return String(value == null ? "" : value);
  }

  function winRate(card) {
    const plays = Math.max(0, finiteNumber(card && card.plays));
    const wins = Math.max(0, finiteNumber(card && card.wins));
    return plays > 0 ? wins / plays : 0;
  }

  function isEligible(card, matchID) {
    if (!card) return false;
    if (card.status === "available") return true;
    return card.status === "match_locked"
      && opaqueID(card.lockRef) === "match:" + opaqueID(matchID);
  }

  function compareCards(left, right) {
    const powerDifference = finiteNumber(right && right.power) - finiteNumber(left && left.power);
    if (powerDifference) return powerDifference;

    const leftRarity = RARITY_WEIGHT[String(left && left.rarity || "").toLowerCase()] || 0;
    const rightRarity = RARITY_WEIGHT[String(right && right.rarity || "").toLowerCase()] || 0;
    if (rightRarity !== leftRarity) return rightRarity - leftRarity;

    const rateDifference = winRate(right) - winRate(left);
    if (rateDifference) return rateDifference;

    const winsDifference = finiteNumber(right && right.wins) - finiteNumber(left && left.wins);
    if (winsDifference) return winsDifference;

    const leftID = opaqueID(left && left.id);
    const rightID = opaqueID(right && right.id);
    if (leftID < rightID) return -1;
    if (leftID > rightID) return 1;
    return 0;
  }

  function rankEligibleCards(cards, matchID) {
    return (Array.isArray(cards) ? cards : [])
      .filter(function (card) { return isEligible(card, matchID); })
      .slice()
      .sort(compareCards);
  }

  function strongestCards(cards, matchID, count) {
    const requestedCount = Math.max(0, Math.floor(finiteNumber(count)));
    return rankEligibleCards(cards, matchID).slice(0, requestedCount);
  }

  return Object.freeze({
    compareCards: compareCards,
    isEligible: isEligible,
    rankEligibleCards: rankEligibleCards,
    strongestCards: strongestCards,
    winRate: winRate,
  });
});
