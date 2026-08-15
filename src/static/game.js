(function () {
  "use strict";

  const MODE_CONFIG = Object.freeze({
    duel: { label: "1 ضد 1", minPlayers: 2, maxPlayers: 2 },
    team_2v2: { label: "2 ضد 2", minPlayers: 4, maxPlayers: 4 },
    team_4v4: { label: "4 ضد 4", minPlayers: 8, maxPlayers: 8 },
    open: { label: "مفتوحة", minPlayers: 2, maxPlayers: 8 },
  });
  const TERMINAL_STATES = new Set(["completed", "forfeited"]);
  let eventsSocket = null;
  let reconnectTimer = null;
  let refreshTimer = null;
  let loadInFlight = false;
  let reloadPending = false;
  let stopped = false;
  let reconnectAttempts = 0;
  let reconcileTimer = null;

  function listPath() {
    const publicRadio = document.getElementById("public_game");
    return publicRadio && publicRadio.checked ? "/api/v1/games/public" : "/api/v1/games/mine";
  }

  function normalizeMode(value) {
    return Object.prototype.hasOwnProperty.call(MODE_CONFIG, value) ? value : "duel";
  }

  function integerBetween(value, minimum, maximum, fallback) {
    const number = Number.parseInt(value, 10);
    return Number.isFinite(number) ? Math.min(maximum, Math.max(minimum, number)) : fallback;
  }

  function gameCapacity(game) {
    const mode = normalizeMode(game && game.mode);
    const config = MODE_CONFIG[mode];
    if (mode !== "open") return config.maxPlayers;
    return integerBetween(game && game.maxPlayers, 2, 8, config.maxPlayers);
  }

  function modeLabel(mode) {
    return MODE_CONFIG[normalizeMode(mode)].label;
  }

  function stateLabel(state, joinable) {
    if (state === "completed") return "مكتملة";
    if (state === "forfeited") return "انسحاب";
    if (state === "tie_break") return "كسر التعادل";
    if (state === "active") return "جارية";
    if (state === "collecting_decks" || state === "prepared") return "تجهيز البطاقات";
    return joinable ? "مفتوحة" : "اكتملت المقاعد";
  }

  function stateDescription(state, members, capacity) {
    if (state === "completed") return "انتهت المنافسة وتم تسجيل الفائز النهائي.";
    if (state === "forfeited") return "انتهت الساحة بالانسحاب.";
    if (state === "tie_break") return "مسابقة مصغّرة جارية لحسم الفائز.";
    if (state === "active") return "الأسئلة جارية والنتيجة تتحدث لحظيًا.";
    if (state === "collecting_decks" || state === "prepared") return "الساحة مجهزة وبانتظار تثبيت مجموعات اللاعبين.";
    if (members >= capacity) return "اكتملت المقاعد؛ ينتظر اللاعبون تجهيز مالك الساحة.";
    return "متاح الآن · " + members + " من " + capacity + " لاعبين";
  }

  function initialFor(user) {
    return ((user && user.fullName) || "؟").trim().charAt(0) || "؟";
  }

  function renderAvatarStack(game, members, capacity) {
    const visual = document.createElement("div");
    visual.className = "qb-lobby-item__visual qb-lobby-avatar-stack";
    visual.setAttribute("aria-label", members.length + " من " + capacity + " لاعبين");

    members.slice(0, capacity).forEach(function (user) {
      const avatar = document.createElement("span");
      const team = Number(user && user.team);
      avatar.className = "qb-player-avatar" + (team > 0 ? " team--" + team : "");
      avatar.textContent = initialFor(user);
      avatar.title = (user.fullName || "لاعب") + (team > 0 ? " · الفريق " + team : "");
      visual.appendChild(avatar);
    });

    for (let index = members.length; index < capacity; index += 1) {
      const empty = document.createElement("span");
      empty.className = "qb-player-avatar qb-player-avatar--empty";
      empty.textContent = "+";
      empty.setAttribute("aria-hidden", "true");
      visual.appendChild(empty);
    }
    return visual;
  }

  function renderBattle(game, isMine) {
    const list = document.getElementById("games_list");
    if (!list || !game || !game.owner) return;

    const joinedUsers = Array.isArray(game.joinedUsers) ? game.joinedUsers : [];
    const members = joinedUsers.length;
    const capacity = gameCapacity(game);
    const state = game.state || "lobby";
    const joinable = Boolean(game.isActive && state === "lobby" && members < capacity);
    const wrapper = document.createElement("article");
    wrapper.className = "media qb-lobby-item state--" + String(state).replace(/_/g, "-");
    wrapper.dataset.gameId = String(game.id);
    wrapper.dataset.mode = normalizeMode(game.mode);

    const description = document.createElement("div");
    description.className = "qb-lobby-item__content";
    const badges = document.createElement("div");
    badges.className = "qb-lobby-item__badges";
    const status = document.createElement("span");
    status.className = "qb-lobby-status";
    status.textContent = stateLabel(state, joinable);
    const mode = document.createElement("span");
    mode.className = "qb-lobby-mode";
    mode.textContent = modeLabel(game.mode);
    badges.append(status, mode);

    const title = document.createElement("strong");
    title.textContent = "ساحة " + game.owner.fullName;
    const details = document.createElement("p");
    details.textContent = stateDescription(state, members, capacity);
    const identifier = document.createElement("small");
    identifier.textContent = "#" + game.id + " · " + members + "/" + capacity + " لاعبين";
    description.append(badges, title, details, identifier);

    const button = document.createElement("button");
    button.type = "button";
    button.className = "qb-button " + (isMine || joinable ? "qb-button--primary" : "qb-button--quiet");
    if (isMine) {
      button.textContent = TERMINAL_STATES.has(state)
        ? "عرض النتيجة"
        : (state === "lobby" ? "فتح الساحة" : "استئناف الساحة");
      button.addEventListener("click", function () { openBattle(game.id); });
    } else {
      button.textContent = joinable ? "انضم إلى الساحة" : "غير متاحة";
      button.disabled = !joinable;
      button.addEventListener("click", function () { joinBattle(game.id); });
    }

    wrapper.append(renderAvatarStack(game, joinedUsers, capacity), description, button);
    list.appendChild(wrapper);
  }

  function renderEmptyBattles(isMine) {
    const list = document.getElementById("games_list");
    if (!list) return;
    const empty = document.createElement("div");
    empty.className = "qb-empty-state qb-empty-state--lobby";
    const title = document.createElement("h3");
    title.textContent = isMine ? "لا توجد ساحات في سجلك بعد" : "لا توجد ساحة عامة مفتوحة الآن";
    const copy = document.createElement("p");
    copy.textContent = isMine
      ? "اختر نوع المنافسة وأنشئ ساحة جديدة؛ ستظهر هنا مع حالتها ونتيجتها."
      : "أنشئ ساحة فردية أو فريقية أو مفتوحة ليتمكن اللاعبون من الانضمام.";
    empty.append(title, copy);
    list.appendChild(empty);
  }

  async function loadBattles() {
    if (loadInFlight) {
      reloadPending = true;
      return;
    }
    const list = document.getElementById("games_list");
    if (!list) return;
    loadInFlight = true;
    list.setAttribute("aria-busy", "true");
    QuizBattle.showError("errorSumm", null);
    try {
      const path = listPath();
      const games = (await QuizBattle.request(path)) || [];
      list.replaceChildren();
      const isMine = path.endsWith("/mine");
      games.forEach(function (game) { renderBattle(game, isMine); });
      if (!games.length) renderEmptyBattles(isMine);
    } catch (error) {
      QuizBattle.showError("errorSumm", error);
    } finally {
      list.setAttribute("aria-busy", "false");
      loadInFlight = false;
      if (reloadPending) {
        reloadPending = false;
        loadBattles();
      }
    }
  }

  function scheduleBattleRefresh() {
    clearTimeout(refreshTimer);
    refreshTimer = setTimeout(loadBattles, 250);
  }

  function selectedMode() {
    const selected = document.querySelector("input[name='gameMode']:checked");
    return normalizeMode(selected && selected.value);
  }

  function selectedCapacity(mode) {
    if (mode !== "open") return MODE_CONFIG[mode].maxPlayers;
    const input = document.getElementById("openMaxPlayers");
    const capacity = integerBetween(input && input.value, 2, 8, 8);
    if (input) input.value = String(capacity);
    return capacity;
  }

  function syncCreateControls() {
    const mode = selectedMode();
    const openControl = document.getElementById("openCapacityControl");
    const openRadio = document.getElementById("gameModeOpen");
    const isOpen = mode === "open";
    if (openControl) openControl.hidden = !isOpen;
    if (openRadio) openRadio.setAttribute("aria-expanded", isOpen ? "true" : "false");
    const button = document.getElementById("creategame");
    if (button) button.textContent = "إنشاء ساحة " + modeLabel(mode);
  }

  async function createBattle(event) {
    if (event) event.preventDefault();
    const button = document.getElementById("creategame");
    if (button) {
      button.disabled = true;
      button.setAttribute("aria-busy", "true");
    }
    try {
      const mode = selectedMode();
      const game = await QuizBattle.request("/api/v1/game", {
        method: "POST",
        body: { isPublic: true, mode: mode, maxPlayers: selectedCapacity(mode) },
      });
      window.location.assign("/battle/" + encodeURIComponent(game.id));
    } catch (error) {
      QuizBattle.showError("errorSumm", error);
    } finally {
      if (button) {
        button.disabled = false;
        button.removeAttribute("aria-busy");
      }
    }
  }

  async function joinBattle(gameID) {
    try {
      await QuizBattle.request("/api/v1/game/" + encodeURIComponent(gameID) + "/join", { method: "POST" });
      window.location.assign("/battle/" + encodeURIComponent(gameID));
    } catch (error) {
      QuizBattle.showError("errorSumm", error);
    }
  }

  function openBattle(gameID) {
    window.location.assign("/battle/" + encodeURIComponent(gameID));
  }

  function connectEvents() {
    if (stopped || !document.getElementById("games_list") || eventsSocket) return;
    eventsSocket = new WebSocket(QuizBattle.websocketURL("/ws/events"));
    eventsSocket.addEventListener("open", function () {
      reconnectAttempts = 0;
      QuizBattle.showError("errorSumm", null);
      loadBattles();
    });
    eventsSocket.addEventListener("message", function (event) {
      try {
        const update = JSON.parse(event.data);
        if (update.type === "sync" || /^\d+$/.test(String(update.gameId))) scheduleBattleRefresh();
      } catch (_) {}
    });
    eventsSocket.addEventListener("close", async function () {
      eventsSocket = null;
      if (stopped) return;
      try {
        await QuizBattle.getSession(true);
      } catch (error) {
        if (error.status === 401) {
          stopped = true;
          window.location.replace("/auth/signin");
          return;
        }
      }
      clearTimeout(reconnectTimer);
      const delay = Math.min(30000, 1000 * Math.pow(2, reconnectAttempts++)) + Math.floor(Math.random() * 500);
      reconnectTimer = setTimeout(connectEvents, delay);
    });
  }

  function stopRealtime() {
    stopped = true;
    clearTimeout(reconnectTimer);
    clearTimeout(refreshTimer);
    clearInterval(reconcileTimer);
    if (eventsSocket) eventsSocket.close();
  }

  window.addEventListener("quizbattle:logout", stopRealtime);
  window.addEventListener("quizbattle:session-invalid", function () {
    stopRealtime();
    window.location.replace("/auth/signin");
  });

  document.addEventListener("DOMContentLoaded", function () {
    if (!document.getElementById("games_list")) return;
    connectEvents();
    loadBattles();
    reconcileTimer = setInterval(function () {
      if (!document.hidden) loadBattles();
    }, 30000);
    const form = document.getElementById("createArenaForm");
    const createButton = document.getElementById("creategame");
    if (form) form.addEventListener("submit", createBattle);
    else if (createButton) createButton.addEventListener("click", createBattle);
    document.querySelectorAll("input[name='gameowner']").forEach(function (input) {
      input.addEventListener("change", loadBattles);
    });
    document.querySelectorAll("input[name='gameMode']").forEach(function (input) {
      input.addEventListener("change", syncCreateControls);
    });
    const capacity = document.getElementById("openMaxPlayers");
    if (capacity) capacity.addEventListener("change", function () { selectedCapacity("open"); });
    syncCreateControls();
  });
})();
