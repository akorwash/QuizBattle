(function () {
  "use strict";

  const maximumRenderedMessages = 100;
  const avatarChangeStorageKey = "quizbattle:avatar-change";
  const messagesByID = new Map();
  const avatarRevisionByUser = new Map();
  const avatarStateByUser = new Map();
  const arabicTime = new Intl.DateTimeFormat("ar-EG", {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
  let socket = null;
  let account = null;
  let reconnectTimer = null;
  let stopped = false;
  let reconnectAttempts = 0;
  let historyLoaded = false;
  let historyInFlight = null;
  let pendingMessageText = null;

  function messageKey(message) {
    if (message && message.id !== undefined && message.id !== null) return "id:" + String(message.id);
    return [message.userId, message.createdAt || "", message.message].map(String).join("\u001f");
  }

  function normalizedMessage(message) {
    if (!message || typeof message.message !== "string" || !message.message.trim()) return null;
    return {
      id: message.id === undefined || message.id === null ? null : String(message.id),
      userId: String(message.userId || ""),
      username: typeof message.username === "string" ? message.username : "",
      fullName: typeof message.fullName === "string" && message.fullName.trim() ? message.fullName.trim() : "لاعب QuizBattle",
      message: message.message.trim(),
      createdAt: typeof message.createdAt === "string" ? message.createdAt : new Date().toISOString(),
    };
  }

  function initialsFor(name) {
    const words = String(name || "")
      .trim()
      .split(/\s+/)
      .filter(Boolean);
    if (!words.length) return "لا";
    if (words.length === 1) return Array.from(words[0]).slice(0, 2).join("");
    return (Array.from(words[0])[0] || "") + (Array.from(words[words.length - 1])[0] || "");
  }

  function avatarURL(userID, revision) {
    if (userID === undefined || userID === null || String(userID).trim() === "") return "";
    const base = "/api/v1/user/avatar/" + encodeURIComponent(String(userID));
    return revision ? base + "?v=" + encodeURIComponent(String(revision)) : base;
  }

  function showAvatarFallback(container) {
    const image = container.querySelector(".qb-chat-avatar__image");
    const fallback = container.querySelector(".qb-chat-avatar__fallback");
    if (image) {
      image.classList.remove("is-loaded");
      image.removeAttribute("src");
    }
    if (fallback) fallback.hidden = false;
  }

  function releaseAvatarState(userID) {
    const key = String(userID || "");
    const state = avatarStateByUser.get(key);
    if (state) state.active = false;
    avatarStateByUser.delete(key);
  }

  function imageDataURL(blob) {
    return new Promise(function (resolve, reject) {
      const reader = new FileReader();
      reader.onload = function () { resolve(String(reader.result || "")); };
      reader.onerror = function () { reject(new Error("could not read avatar image")); };
      reader.readAsDataURL(blob);
    });
  }

  function cachedAvatarState(userID, revision, missing) {
    const key = String(userID || "");
    const normalizedRevision = String(revision || "");
    if (!key) return Promise.resolve({ status: "missing", revision: normalizedRevision });

    const existing = avatarStateByUser.get(key);
    if (existing && existing.revision === normalizedRevision && existing.status !== "error") {
      return existing.promise;
    }
    releaseAvatarState(key);

    if (missing) {
      const missingState = { status: "missing", revision: normalizedRevision, imageSource: "", active: true };
      missingState.promise = Promise.resolve(missingState);
      avatarStateByUser.set(key, missingState);
      return missingState.promise;
    }

    const state = { status: "loading", revision: normalizedRevision, imageSource: "", active: true, promise: null };
    state.promise = fetch(avatarURL(key, normalizedRevision), {
      credentials: "same-origin",
      headers: { Accept: "image/jpeg,image/png,image/*" },
      cache: "no-cache",
    }).then(async function (response) {
      if (response.status === 404) {
        state.status = "missing";
        return state;
      }
      if (response.status === 401) window.dispatchEvent(new Event("quizbattle:session-invalid"));
      if (!response.ok) throw new Error("avatar request failed with status " + response.status);
      const blob = await response.blob();
      if (!blob.type.startsWith("image/")) throw new Error("avatar response is not an image");
      const source = await imageDataURL(blob);
      if (!state.active || avatarStateByUser.get(key) !== state) return state;
      state.imageSource = source;
      state.status = "ready";
      return state;
    }).catch(function () {
      state.status = "error";
      return state;
    });
    avatarStateByUser.set(key, state);
    return state.promise;
  }

  function loadAvatar(container, userID, fullName, revision, deleted) {
    const image = container.querySelector(".qb-chat-avatar__image");
    const fallback = container.querySelector(".qb-chat-avatar__fallback");
    if (!image || !fallback) return;

    const key = String(userID || "");
    const normalizedRevision = String(revision || "");
    const requestKey = key + "\u001f" + normalizedRevision;
    container.dataset.avatarUserId = key;
    container.dataset.avatarFullName = String(fullName || "");
    container.dataset.avatarRequestKey = requestKey;
    fallback.textContent = initialsFor(fullName);
    image.onload = null;
    image.onerror = null;
    showAvatarFallback(container);
    cachedAvatarState(key, normalizedRevision, deleted).then(function (state) {
      if (container.dataset.avatarRequestKey !== requestKey || state.status !== "ready" || !state.imageSource) return;
      image.onload = function () {
        image.classList.add("is-loaded");
        fallback.hidden = true;
      };
      image.onerror = function () {
        showAvatarFallback(container);
      };
      image.src = state.imageSource;
    });
  }

  function createAvatar(userID, fullName) {
    const container = document.createElement("div");
    container.className = "qb-chat-avatar";
    const image = document.createElement("img");
    image.className = "qb-chat-avatar__image";
    image.alt = "";
    image.width = 48;
    image.height = 48;
    image.loading = "lazy";
    image.decoding = "async";
    const fallback = document.createElement("span");
    fallback.className = "qb-chat-avatar__fallback";
    fallback.setAttribute("aria-hidden", "true");
    container.append(image, fallback);
    loadAvatar(container, userID, fullName, avatarRevisionByUser.get(String(userID || "")), false);
    return container;
  }

  function refreshAvatars(detail) {
    const userID = detail && detail.userId !== undefined && detail.userId !== null ? String(detail.userId) : "";
    if (!userID) return;
    const revision = detail.revision || detail.etag || "";
    if (revision) avatarRevisionByUser.set(userID, String(revision));
    releaseAvatarState(userID);
    if (detail.hasAvatar === false) cachedAvatarState(userID, revision, true);
    document.querySelectorAll(".qb-chat-avatar[data-avatar-user-id]").forEach(function (container) {
      if (container.dataset.avatarUserId !== userID) return;
      loadAvatar(container, userID, container.dataset.avatarFullName, revision, detail.hasAvatar === false);
    });
  }

  function createMessageRow(message) {
    const ownMessage = account && String(message.userId) === String(account.userId);
    const row = document.createElement("article");
    row.className = "chat_list" + (ownMessage ? " is-own-message" : "");
    if (message.id) row.dataset.messageId = message.id;

    const people = document.createElement("div");
    people.className = "chat_people";
    const imageContainer = document.createElement("div");
    imageContainer.className = (ownMessage ? "chat_img_left" : "chat_img_right") + " qb-chat-avatar-shell";
    imageContainer.appendChild(createAvatar(message.userId, message.fullName));

    const body = document.createElement("div");
    body.className = "chat_ib";
    const metadata = document.createElement("div");
    metadata.className = "qb-chat-message__meta";
    const name = document.createElement("h5");
    name.textContent = message.fullName + (message.username ? " @" + message.username : "");
    const timestamp = document.createElement("time");
    timestamp.className = "qb-chat-message__time";
    const date = new Date(message.createdAt);
    if (!Number.isNaN(date.getTime())) {
      timestamp.dateTime = date.toISOString();
      timestamp.textContent = arabicTime.format(date);
    }
    metadata.append(name, timestamp);
    body.appendChild(metadata);
    message.message.split("\n").forEach(function (line) {
      const paragraph = document.createElement("p");
      paragraph.textContent = line || " ";
      body.appendChild(paragraph);
    });
    people.append(imageContainer, body);
    row.appendChild(people);
    return row;
  }

  function sortedMessages() {
    return Array.from(messagesByID.values()).sort(function (left, right) {
      const timeDifference = new Date(left.createdAt).getTime() - new Date(right.createdAt).getTime();
      if (timeDifference !== 0) return timeDifference;
      return String(left.id || "").localeCompare(String(right.id || ""));
    }).slice(-maximumRenderedMessages);
  }

  function renderHistory() {
    const list = document.getElementById("messages");
    if (!list) return;
    list.setAttribute("aria-busy", "true");
    list.setAttribute("aria-live", "off");
    list.replaceChildren();
    const messages = sortedMessages();
    if (!messages.length) {
      const empty = document.createElement("p");
      empty.className = "qb-chat-empty";
      empty.textContent = "لا توجد رسائل محفوظة بعد. ابدأ حديثًا محترمًا مع المجتمع.";
      list.appendChild(empty);
    } else {
      messages.forEach(function (message) { list.appendChild(createMessageRow(message)); });
    }
    list.scrollTop = list.scrollHeight;
    list.removeAttribute("aria-busy");
    window.requestAnimationFrame(function () { list.setAttribute("aria-live", "polite"); });
  }

  function appendLiveMessage(rawMessage) {
    const message = normalizedMessage(rawMessage);
    if (!message) return;
    if (pendingMessageText !== null && account && sameUser(message.userId, account.userId) && message.message === pendingMessageText) {
      const input = document.getElementById("inputmessage");
      if (input && input.value.trim() === pendingMessageText) input.value = "";
      pendingMessageText = null;
      setConnectionState(true);
    }
    const key = messageKey(message);
    if (messagesByID.has(key)) return;
    messagesByID.set(key, message);
    if (!historyLoaded) return;
    const list = document.getElementById("messages");
    if (!list) return;
    const keepAtBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 80;
    const empty = list.querySelector(".qb-chat-empty");
    if (empty) empty.remove();
    list.appendChild(createMessageRow(message));
    while (list.children.length > maximumRenderedMessages) list.firstElementChild.remove();
    if (keepAtBottom) list.scrollTop = list.scrollHeight;
  }

  function loadHistory() {
    if (historyInFlight) return historyInFlight;
    historyInFlight = QuizBattle.request("/api/v1/chat/messages")
      .then(function (messages) {
        (Array.isArray(messages) ? messages : []).forEach(function (rawMessage) {
          const message = normalizedMessage(rawMessage);
          if (message) messagesByID.set(messageKey(message), message);
        });
        historyLoaded = true;
        renderHistory();
      })
      .catch(function (error) {
        historyLoaded = true;
        renderHistory();
        QuizBattle.showError("worldchaterrorSumm", error);
      })
      .finally(function () { historyInFlight = null; });
    return historyInFlight;
  }

  function setConnectionState(connected) {
    const button = document.getElementById("sendmessage");
    if (button) {
      button.disabled = !connected || !account || pendingMessageText !== null;
      button.textContent = pendingMessageText !== null ? "جارٍ الحفظ…" : (connected ? "إرسال" : "إعادة الاتصال…");
    }
  }

  function sameUser(left, right) {
    return String(left) === String(right);
  }

  function connect() {
    if (stopped || !document.getElementById("messages") || socket) return;
    setConnectionState(false);
    socket = new WebSocket(QuizBattle.websocketURL("/ws/world-chat"));
    socket.addEventListener("open", function () {
      reconnectAttempts = 0;
      setConnectionState(true);
      QuizBattle.showError("worldchaterrorSumm", null);
      loadHistory();
    });
    socket.addEventListener("message", function (event) {
      try {
        const message = JSON.parse(event.data);
        if (message.type === "text") appendLiveMessage(message);
      } catch (_) {}
    });
    socket.addEventListener("close", async function () {
      socket = null;
      pendingMessageText = null;
      setConnectionState(false);
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
      QuizBattle.showError("worldchaterrorSumm", new Error("انقطع اتصال المحادثة، وجارٍ إعادة الاتصال."));
      clearTimeout(reconnectTimer);
      const delay = Math.min(30000, 1000 * Math.pow(2, reconnectAttempts++)) + Math.floor(Math.random() * 500);
      reconnectTimer = setTimeout(connect, delay);
    });
  }

  function stopRealtime() {
    stopped = true;
    clearTimeout(reconnectTimer);
    setConnectionState(false);
    if (socket) socket.close();
  }

  function sendMessage() {
    const input = document.getElementById("inputmessage");
    const text = input ? input.value.trim() : "";
    if (!text || pendingMessageText !== null) return;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      QuizBattle.showError("worldchaterrorSumm", new Error("المحادثة غير متصلة الآن. احتفظ برسالتك وحاول بعد لحظة."));
      return;
    }
    try {
      pendingMessageText = text;
      setConnectionState(true);
      socket.send(JSON.stringify({ type: "text", message: text }));
    } catch (error) {
      pendingMessageText = null;
      setConnectionState(false);
      QuizBattle.showError("worldchaterrorSumm", error);
    }
  }

  window.addEventListener("quizbattle:logout", stopRealtime);
  window.addEventListener("quizbattle:session-invalid", stopRealtime);
  window.addEventListener("quizbattle:avatar-changed", function (event) {
    refreshAvatars(event.detail || {});
  });
  window.addEventListener("storage", function (event) {
    if (event.key !== avatarChangeStorageKey || !event.newValue) return;
    try {
      refreshAvatars(JSON.parse(event.newValue));
    } catch (_) {}
  });
  window.addEventListener("pagehide", function (event) {
    if (event.persisted) return;
    Array.from(avatarStateByUser.keys()).forEach(releaseAvatarState);
  });
  window.addEventListener("quizbattle:session-changed", function (event) {
    account = event.detail || account;
    if (socket) socket.close();
  });

  document.addEventListener("DOMContentLoaded", function () {
    const list = document.getElementById("messages");
    if (!list) return;
    list.replaceChildren();
    list.setAttribute("aria-busy", "true");
    setConnectionState(false);
    QuizBattle.getSession()
      .then(function (value) {
        account = value;
        setConnectionState(Boolean(socket && socket.readyState === WebSocket.OPEN));
        return loadHistory();
      })
      .catch(function (error) { QuizBattle.showError("worldchaterrorSumm", error); });
    connect();
    const button = document.getElementById("sendmessage");
    if (button) button.addEventListener("click", sendMessage);
    const input = document.getElementById("inputmessage");
    if (input) {
      input.maxLength = 500;
      input.addEventListener("keydown", function (event) {
        if (event.key === "Enter" && !event.shiftKey) {
          event.preventDefault();
          sendMessage();
        }
      });
    }
  });
})();
