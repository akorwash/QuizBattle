(function () {
  "use strict";

  let account = null;
  let collection = null;

  function commandID(prefix) {
    if (window.crypto && typeof window.crypto.randomUUID === "function") {
      return prefix + "-" + window.crypto.randomUUID();
    }
    const random = Math.floor(Math.random() * 0x7fffffff).toString(16);
    return prefix + "-" + Date.now().toString(16) + "-" + random;
  }

  function setText(id, value) {
    const element = document.getElementById(id);
    if (element) element.textContent = value;
  }

  function rarityLabel(value) {
    return window.QuizBattleCardVisuals
      ? QuizBattleCardVisuals.rarityLabel(value)
      : ({ common: "شائعة", rare: "نادرة", epic: "ملحمية", legendary: "أسطورية" }[value] || value);
  }

  function categoryLabel(value) {
    return window.QuizBattleCardVisuals
      ? QuizBattleCardVisuals.categoryLabel(value)
      : value;
  }

  function difficultyLabel(value) {
    return window.QuizBattleCardVisuals
      ? QuizBattleCardVisuals.difficultyLabel(value)
      : value;
  }

  function statusLabel(value) {
    return {
      available: "متاحة",
      match_locked: "داخل مباراة",
      market_escrow: "معروضة للبيع",
      trade_escrow: "محجوزة للتبادل",
      active: "نشط",
      sold: "مباع",
      cancelled: "ملغي",
      pending: "بانتظار الرد",
      accepted: "مقبول",
      rejected: "مرفوض",
      expired: "منتهي",
    }[value] || value;
  }

  function cardArticle(card, options) {
    const settings = options || {};
    const context = settings.context || "collection";
    const article = document.createElement("article");
    article.className = "qb-game-card qb-collection-card rarity--" + card.rarity;
    article.classList.add("status--" + String(card.status || "unknown").replaceAll("_", "-"));
    if (context === "collection" && card.status !== "available") article.classList.add("is-locked");

    const art = window.QuizBattleCardVisuals
      ? QuizBattleCardVisuals.createArt(card.category)
      : document.createElement("div");
    art.classList.add("qb-game-card__art");
    const status = document.createElement("span");
    status.className = "qb-card-status";
    status.textContent = settings.statusText || statusLabel(card.status);
    art.appendChild(status);

    const body = document.createElement("div");
    body.className = "qb-game-card__body";

    const header = document.createElement("div");
    header.className = "qb-card__header";
    const category = document.createElement("span");
    category.className = "qb-card__category";
    category.textContent = categoryLabel(card.category);
    const rarity = document.createElement("span");
    rarity.className = "qb-card__rarity";
    rarity.textContent = rarityLabel(card.rarity);
    header.append(category, rarity);

    const title = document.createElement("h3");
    title.textContent = card.prompt;

    const stats = document.createElement("div");
    stats.className = "qb-card-stats";
    [
      ["المستوى", card.power],
      ["الصعوبة", difficultyLabel(card.difficulty)],
      ["لعب", card.plays],
      ["فوز", card.wins],
    ].forEach(function (entry) {
      const item = document.createElement("span");
      const value = document.createElement("strong");
      const label = document.createElement("small");
      value.textContent = String(entry[1]);
      label.textContent = entry[0];
      item.append(value, label);
      stats.appendChild(item);
    });

    const source = document.createElement("small");
    source.className = "qb-card-source";
    source.textContent = (card.sourceTitle ? card.sourceTitle + " · " : "") + "بطاقة #" + card.id;
    body.append(header, title, stats, source);
    article.append(art, body);

    if (settings.allowSell && card.status === "available") {
      const form = document.createElement("form");
      form.className = "qb-inline-action";
      const price = document.createElement("input");
      price.type = "number";
      price.min = "10";
      price.max = "100000";
      price.value = "100";
      price.className = "form-control";
      price.setAttribute("aria-label", "سعر البطاقة بالعملات");
      const submit = document.createElement("button");
      submit.type = "submit";
      submit.className = "qb-button qb-button--brass";
      submit.textContent = "اعرض للبيع";
      form.append(price, submit);
      form.addEventListener("submit", async function (event) {
        event.preventDefault();
        submit.disabled = true;
        try {
          await QuizBattle.request("/api/v1/market/listings", {
            method: "POST",
            body: { cardId: card.id, price: Number(price.value), commandId: commandID("list") },
          });
          await loadAll();
        } catch (error) {
          QuizBattle.showError("accountError", error);
        } finally {
          submit.disabled = false;
        }
      });
      body.appendChild(form);
    }
    return article;
  }

  function visibleCollectionCards(cards) {
    const searchInput = document.getElementById("cardSearch");
    const categoryInput = document.getElementById("cardCategoryFilter");
    const statusInput = document.getElementById("cardStatusFilter");
    const query = searchInput ? searchInput.value.trim().toLocaleLowerCase("ar") : "";
    const category = categoryInput ? categoryInput.value : "";
    const status = statusInput ? statusInput.value : "";
    return cards.filter(function (card) {
      const matchesQuery = !query || [card.prompt, card.sourceTitle, card.id]
        .some(function (value) { return String(value || "").toLocaleLowerCase("ar").includes(query); });
      return matchesQuery && (!category || card.category === category) && (!status || card.status === status);
    });
  }

  function renderCollection(value) {
    collection = value;
    setText("ownedCardsMetric", String(value.cards.length));
    setText("walletMetric", String(value.wallet.available));
    setText("lockedCoinsMetric", String(value.wallet.locked));
    const visibleCards = visibleCollectionCards(value.cards);
    setText("collectionCount", visibleCards.length === value.cards.length
      ? value.cards.length + " بطاقة"
      : visibleCards.length + " من " + value.cards.length + " بطاقة");
    const container = document.getElementById("cardCollection");
    if (!container) return;
    container.replaceChildren();
    if (!value.cards.length) {
      const empty = document.createElement("p");
      empty.className = "qb-help";
      empty.textContent = "لا توجد بطاقات في المجموعة.";
      container.appendChild(empty);
      return;
    }
    if (!visibleCards.length) {
      const empty = document.createElement("div");
      empty.className = "qb-empty-state";
      const message = document.createElement("p");
      message.textContent = "لا توجد بطاقات تطابق البحث أو الفلاتر الحالية.";
      empty.appendChild(message);
      container.appendChild(empty);
      return;
    }
    visibleCards.forEach(function (card) {
      container.appendChild(cardArticle(card, { allowSell: true, context: "collection" }));
    });
  }

  function renderMarket(listings) {
    setText("marketStatus", listings.length + " عرض متاح");
    const container = document.getElementById("marketplaceList");
    if (!container) return;
    container.className = "qb-card-grid";
    container.replaceChildren();
    if (!listings.length) {
      const empty = document.createElement("p");
      empty.className = "qb-help";
      empty.textContent = "لا توجد بطاقات معروضة حاليًا. يمكنك عرض بطاقة متاحة من مجموعتك.";
      container.appendChild(empty);
      return;
    }
    listings.forEach(function (listing) {
      const mine = String(listing.sellerId) === String(account.userId);
      const article = cardArticle(listing.card, {
        context: "market",
        statusText: mine ? "عرضك في السوق" : "متاحة للشراء",
      });
      const cardBody = article.querySelector(".qb-game-card__body") || article;
      const purchase = document.createElement("div");
      purchase.className = "qb-market-action";
      const price = document.createElement("strong");
      price.className = "qb-price";
      price.textContent = listing.price + " عملة";
      const action = document.createElement("button");
      action.className = "qb-button qb-button--primary";
      action.type = "button";
      action.textContent = mine ? "إلغاء العرض" : "شراء البطاقة";
      action.addEventListener("click", async function () {
        action.disabled = true;
        try {
          const suffix = mine ? "/cancel" : "/buy";
          await QuizBattle.request("/api/v1/market/listings/" + listing.id + suffix, {
            method: "POST",
            body: { commandId: commandID(mine ? "cancel-listing" : "buy") },
          });
          await loadAll();
        } catch (error) {
          QuizBattle.showError("accountError", error);
        } finally {
          action.disabled = false;
        }
      });
      purchase.append(price, action);
      cardBody.appendChild(purchase);
      container.appendChild(article);
    });
  }

  function renderTrades(trades) {
    setText("tradeStatus", trades.filter(function (trade) { return trade.status === "pending"; }).length + " عرض معلّق");
    const container = document.getElementById("tradeOffersList");
    if (!container) return;
    container.className = "qb-trade-list";
    container.replaceChildren();
    if (!trades.length) {
      const empty = document.createElement("p");
      empty.className = "qb-help";
      empty.textContent = "لا توجد عروض تداول بعد.";
      container.appendChild(empty);
      return;
    }
    trades.forEach(function (trade) {
      const article = document.createElement("article");
      article.className = "qb-trade-offer";
      const title = document.createElement("h3");
      const incoming = String(trade.receiverId) === String(account.userId);
      title.textContent = (incoming ? "عرض وارد من " : "عرض صادر إلى ") + (incoming ? trade.senderId : trade.receiverId);
      const summary = document.createElement("p");
      summary.textContent = "يعرض " + trade.offeredCardIds.length + " بطاقة و" + trade.offeredCoins + " عملة، مقابل " + trade.requestedCardIds.length + " بطاقة و" + trade.requestedCoins + " عملة.";
      const state = document.createElement("span");
      state.className = "qb-card__rarity";
      state.textContent = statusLabel(trade.status);
      article.append(title, summary, state);
      if (trade.status === "pending") {
        const actions = document.createElement("div");
        actions.className = "qb-actions";
        if (incoming) actions.appendChild(tradeButton(trade, "accept", "قبول", "qb-button--primary"));
        actions.appendChild(tradeButton(trade, incoming ? "reject" : "cancel", incoming ? "رفض" : "إلغاء", "qb-button--danger"));
        article.appendChild(actions);
      }
      container.appendChild(article);
    });
  }

  function tradeButton(trade, action, label, className) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "qb-button " + className;
    button.textContent = label;
    button.addEventListener("click", async function () {
      button.disabled = true;
      try {
        await QuizBattle.request("/api/v1/trades/" + trade.id + "/" + action, {
          method: "POST", body: { commandId: commandID("trade-" + action) },
        });
        await loadAll();
      } catch (error) {
        QuizBattle.showError("tradeError", error);
      } finally {
        button.disabled = false;
      }
    });
    return button;
  }

  function parseIDList(value) {
    if (!value.trim()) return [];
    const values = value.split(",").map(function (item) { return item.trim(); });
    if (values.some(function (item) { return !/^\d+$/.test(item); })) throw new Error("أرقام البطاقات يجب أن تكون أرقامًا مفصولة بفواصل.");
    return values;
  }

  async function createTrade(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const submit = form.querySelector('button[type="submit"]');
    if (submit) submit.disabled = true;
    form.setAttribute("aria-busy", "true");
    QuizBattle.showError("tradeError", null);
    try {
      const receiver = document.getElementById("tradeReceiver").value.trim();
      if (!/^\d+$/.test(receiver)) throw new Error("أدخل رقم لاعب صحيحًا.");
      await QuizBattle.request("/api/v1/trades", {
        method: "POST",
        body: {
          receiverId: receiver,
          offeredCardIds: parseIDList(document.getElementById("tradeOfferedCards").value),
          requestedCardIds: parseIDList(document.getElementById("tradeRequestedCards").value),
          offeredCoins: Number(document.getElementById("tradeOfferedCoins").value || 0),
          requestedCoins: Number(document.getElementById("tradeRequestedCoins").value || 0),
          commandId: commandID("trade-create"),
        },
      });
      form.reset();
      document.getElementById("tradeOfferedCoins").value = "0";
      document.getElementById("tradeRequestedCoins").value = "0";
      await loadAll();
    } catch (error) {
      QuizBattle.showError("tradeError", error);
    } finally {
      form.removeAttribute("aria-busy");
      if (submit) submit.disabled = false;
    }
  }

  async function loadAll() {
    QuizBattle.showError("accountError", null);
    try {
      const results = await Promise.all([
        QuizBattle.request("/api/v1/collection"),
        QuizBattle.request("/api/v1/market"),
        QuizBattle.request("/api/v1/trades"),
      ]);
      renderCollection(results[0]);
      renderMarket(results[1]);
      renderTrades(results[2]);
    } catch (error) {
      QuizBattle.showError("accountError", error);
    }
  }

  document.addEventListener("DOMContentLoaded", async function () {
    if (!document.getElementById("cardCollection")) return;
    try {
      account = await QuizBattle.getSession();
      await loadAll();
    } catch (error) {
      QuizBattle.showError("accountError", error);
    }
    const form = document.getElementById("createTradeForm");
    if (form) form.addEventListener("submit", createTrade);
    ["cardSearch", "cardCategoryFilter", "cardStatusFilter"].forEach(function (id) {
      const control = document.getElementById(id);
      if (!control) return;
      control.addEventListener(control.tagName === "INPUT" ? "input" : "change", function () {
        if (collection) renderCollection(collection);
      });
    });
  });
})();
