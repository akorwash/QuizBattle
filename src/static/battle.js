(function () {
  "use strict";

  let battleID = null;
  let battleSocket = null;
  let currentGame = null;
  let currentAccount = null;
  let currentCollection = null;
  let currentMatch = null;
  let loadInFlight = false;
  let loadPending = false;
  let stopped = false;
  let reconnectTimer = null;
  let reconnectAttempts = 0;
  let pollTimer = null;
  let clockTimer = null;
  let lastRenderedTurnKey = null;
  let lastLogKey = null;
  let answerSubmittingTurnID = null;
  let lastRevealedTurnID = null;
  let lastResultFocusKey = null;
  let collectionRefreshKey = null;
  const selectedCards = new Set();
  let selectedDeckSnapshotKey = null;
  let deckAssistMessage = "اختر يدويًا أو دع النظام يقترح أقوى خمس بطاقات.";
  const MODE_CONFIG = Object.freeze({
    duel: { label: "1 ضد 1", minPlayers: 2, maxPlayers: 2, teamSize: 1 },
    team_2v2: { label: "2 ضد 2", minPlayers: 4, maxPlayers: 4, teamSize: 2 },
    team_4v4: { label: "4 ضد 4", minPlayers: 8, maxPlayers: 8, teamSize: 4 },
    open: { label: "ساحة مفتوحة", minPlayers: 2, maxPlayers: 8, teamSize: 0 },
    bot: { label: "ضد البوت", minPlayers: 2, maxPlayers: 2, teamSize: 1 },
  });
  const VOICE_SIGNAL_TYPES = new Set(["voice_ready", "voice_leave", "voice_offer", "voice_answer", "voice_ice"]);
  let voiceJoined = false;
  let voiceJoining = false;
  let voiceMuted = false;
  let voiceJoinAttempt = 0;
  let localVoiceStream = null;
  let voicePeer = null;
  let voiceRemoteUserID = null;
  let voiceRemoteReady = false;
  let announcedVoiceReadyFor = null;
  let pendingVoiceOffer = null;
  let pendingVoiceCandidates = [];
  let pendingVoiceSignals = [];
  let voiceOperation = Promise.resolve();

  function commandID(prefix) {
    if (window.crypto && typeof window.crypto.randomUUID === "function") return prefix + "-" + window.crypto.randomUUID();
    return prefix + "-" + Date.now().toString(16) + "-" + Math.floor(Math.random() * 0x7fffffff).toString(16);
  }

  function parseBattleID() {
    const value = window.location.pathname.split("/").filter(Boolean).pop();
    return /^\d+$/.test(value || "") ? value : null;
  }

  function sameID(left, right) {
    return String(left) === String(right);
  }

  function normalizeMode(value) {
    return Object.prototype.hasOwnProperty.call(MODE_CONFIG, value) ? value : "duel";
  }

  function gameMode() {
    return normalizeMode(currentGame && currentGame.mode);
  }

  function modeConfig() {
    const mode = gameMode();
    const fallback = MODE_CONFIG[mode];
    const maximum = Number(currentGame && currentGame.maxPlayers);
    const minimum = Number(currentGame && currentGame.minPlayers);
    return {
      mode: mode,
      label: fallback.label,
      minPlayers: Number.isFinite(minimum) && minimum >= 2 ? minimum : fallback.minPlayers,
      maxPlayers: Number.isFinite(maximum) && maximum >= 2 && maximum <= 8 ? maximum : fallback.maxPlayers,
      teamSize: Number(currentGame && currentGame.teamSize) || fallback.teamSize,
    };
  }

  function isDuelMode() {
    return gameMode() === "duel" || isBotMode();
  }

  function isBotMode() {
    return Boolean(currentGame && (gameMode() === "bot" || currentGame.opponentType === "bot" ||
      joinedUsers().some(function (user) { return user && user.isBot; })));
  }

  function isTeamMode() {
    return gameMode() === "team_2v2" || gameMode() === "team_4v4";
  }

  function isOwner() {
    return Boolean(currentGame && currentAccount && currentGame.owner && sameID(currentGame.owner.id, currentAccount.userId));
  }

  function joinedUsers() {
    return currentGame && Array.isArray(currentGame.joinedUsers) ? currentGame.joinedUsers : [];
  }

  function matchPlayers() {
    return currentMatch && Array.isArray(currentMatch.players) ? currentMatch.players : [];
  }

  function userForID(userID) {
    return joinedUsers().find(function (user) { return sameID(user.id, userID); }) || null;
  }

  function playerName(userID) {
    const user = userForID(userID);
    return user && user.fullName ? user.fullName : "اللاعب #" + userID;
  }

  function botStrategyLabel(value) {
    return value === "random" ? "عشوائي" : "ذكي";
  }

  function teamLabel(value) {
    const team = Number(value);
    return team > 0 ? "الفريق " + team : "بدون فريق";
  }

  function initialForName(value) {
    return String(value || "؟").trim().charAt(0) || "؟";
  }

  function acceptMatchSnapshot(nextMatch) {
    if (!nextMatch) {
      if (currentMatch) return false;
      currentMatch = null;
      return true;
    }
    const nextVersion = Number(nextMatch.version);
    const currentVersion = Number(currentMatch && currentMatch.version);
    if (currentMatch && Number.isFinite(nextVersion) && Number.isFinite(currentVersion) && nextVersion < currentVersion) {
      return false;
    }
    currentMatch = nextMatch;
    return true;
  }

  function categoryLabel(value) {
    return window.QuizBattleCardVisuals ? QuizBattleCardVisuals.categoryLabel(value) : value;
  }

  function difficultyLabel(value) {
    return window.QuizBattleCardVisuals ? QuizBattleCardVisuals.difficultyLabel(value) : value;
  }

  function rarityLabel(value) {
    return window.QuizBattleCardVisuals ? QuizBattleCardVisuals.rarityLabel(value) : value;
  }

  function setText(id, value) {
    const element = document.getElementById(id);
    if (element) element.textContent = value;
  }

  function setVoiceStatus(message, state) {
    setText("voiceStatus", message);
    const dot = document.getElementById("voiceStatusDot");
    if (dot) dot.dataset.state = state || "idle";
  }

  function renderVoiceControls() {
    const panel = document.getElementById("voiceChatPanel");
    const join = document.getElementById("joinVoiceButton");
    const mute = document.getElementById("muteVoiceButton");
    const leave = document.getElementById("leaveVoiceButton");
    const botMode = isBotMode();
    const unsupportedMode = Boolean(currentGame && (!isDuelMode() || botMode));
    if (panel) {
      panel.hidden = botMode;
      panel.setAttribute("aria-hidden", botMode ? "true" : "false");
    }
    if (join) {
      join.hidden = voiceJoined;
      join.disabled = unsupportedMode || voiceJoined || voiceJoining || !currentAccount || stopped;
      join.setAttribute("aria-busy", voiceJoining ? "true" : "false");
      join.textContent = botMode ? "الصوت غير متاح ضد البوت" : (unsupportedMode ? "الصوت متاح في 1 ضد 1 فقط" : (voiceJoining ? "بانتظار إذن الميكروفون…" : "انضم إلى الصوت"));
    }
    if (mute) {
      mute.hidden = !voiceJoined;
      mute.disabled = !localVoiceStream;
      mute.setAttribute("aria-pressed", voiceMuted ? "true" : "false");
      mute.textContent = voiceMuted ? "إلغاء كتم الميكروفون" : "كتم الميكروفون";
    }
    if (leave) leave.hidden = !voiceJoined;
    if (unsupportedMode && !voiceJoined && !voiceJoining) setVoiceStatus("الدردشة الصوتية معطلة في الساحات الجماعية حاليًا.", "idle");
  }

  function currentOpponentID() {
    if (!isDuelMode() || isBotMode() || !currentAccount || !currentGame || !Array.isArray(currentGame.joinedUsers)) return null;
    const opponent = currentGame.joinedUsers.find(function (user) {
      return user && !sameID(user.id, currentAccount.userId);
    });
    return opponent ? opponent.id : null;
  }

  function isExpectedVoiceSender(userID) {
    const opponentID = currentOpponentID();
    return userID !== undefined && userID !== null && opponentID !== null && sameID(userID, opponentID);
  }

  function compareUserIDs(left, right) {
    const leftID = String(left).replace(/^0+(?=\d)/, "");
    const rightID = String(right).replace(/^0+(?=\d)/, "");
    if (leftID.length !== rightID.length) return leftID.length - rightID.length;
    return leftID.localeCompare(rightID, "en");
  }

  function shouldInitiateVoice(remoteUserID) {
    return currentAccount && remoteUserID !== null && compareUserIDs(currentAccount.userId, remoteUserID) < 0;
  }

  function configuredVoiceIceServers() {
    if (window.QuizBattleVoiceConfig && Array.isArray(window.QuizBattleVoiceConfig.iceServers)) {
      return window.QuizBattleVoiceConfig.iceServers;
    }
    const meta = document.querySelector('meta[name="quizbattle-voice-stun"]');
    const urls = meta && meta.content
      ? meta.content.split(/[\s,]+/).map(function (value) { return value.trim(); }).filter(Boolean)
      : [];
    return urls.length ? [{ urls: urls }] : [];
  }

  function sendVoiceSignal(type, payload) {
    if (!VOICE_SIGNAL_TYPES.has(type) || !battleSocket || battleSocket.readyState !== WebSocket.OPEN) return false;
    try {
      // The server derives the sender and arena from the authenticated socket; never trust client-supplied IDs.
      battleSocket.send(JSON.stringify({ type: type, payload: payload || {} }));
      return true;
    } catch (_) {
      return false;
    }
  }

  function announceVoiceReady(force) {
    if (!voiceJoined) return false;
    const opponentID = currentOpponentID();
    const key = opponentID === null ? "waiting" : String(opponentID);
    if (!force && announcedVoiceReadyFor === key) return true;
    const sent = sendVoiceSignal("voice_ready", {});
    if (sent) announcedVoiceReadyFor = key;
    return sent;
  }

  function stopLocalVoiceStream() {
    if (localVoiceStream) {
      localVoiceStream.getTracks().forEach(function (track) { track.stop(); });
      localVoiceStream = null;
    }
  }

  function closeVoicePeer() {
    const peer = voicePeer;
    voicePeer = null;
    if (peer) {
      peer.onicecandidate = null;
      peer.ontrack = null;
      peer.onconnectionstatechange = null;
      peer.oniceconnectionstatechange = null;
      peer.close();
    }
    const audio = document.getElementById("remoteVoiceAudio");
    if (audio) {
      audio.pause();
      audio.srcObject = null;
    }
    const resume = document.getElementById("resumeVoiceButton");
    if (resume) resume.hidden = true;
  }

  function resetRemoteVoice(message) {
    closeVoicePeer();
    voiceRemoteReady = false;
    pendingVoiceOffer = null;
    pendingVoiceCandidates = [];
    if (voiceJoined) setVoiceStatus(message || "الميكروفون جاهز. بانتظار انضمام المنافس للصوت.", "waiting");
  }

  function updateVoiceConnectionStatus() {
    if (!voiceJoined) return;
    const state = voicePeer ? voicePeer.connectionState : "new";
    if (state === "connected") {
      setVoiceStatus(voiceMuted ? "الصوت متصل والميكروفون مكتوم." : "الصوت متصل بالمنافس مباشرة.", voiceMuted ? "muted" : "connected");
    } else if (["connecting", "new"].includes(state) && voiceRemoteReady) {
      setVoiceStatus("جاري إنشاء الاتصال الصوتي الآمن…", "connecting");
    }
  }

  function createVoicePeer() {
    if (voicePeer) return voicePeer;
    if (!localVoiceStream) throw new Error("local voice stream is unavailable");

    // Restricted networks need TURN with short-lived, server-issued credentials in production.
    // Never embed persistent TURN credentials in this public JavaScript bundle.
    const peer = new RTCPeerConnection({ iceServers: configuredVoiceIceServers() });
    voicePeer = peer;
    localVoiceStream.getAudioTracks().forEach(function (track) {
      peer.addTrack(track, localVoiceStream);
    });
    peer.addEventListener("icecandidate", function (event) {
      if (!event.candidate || peer !== voicePeer || !voiceJoined) return;
      const candidate = typeof event.candidate.toJSON === "function"
        ? event.candidate.toJSON()
        : {
            candidate: event.candidate.candidate,
            sdpMid: event.candidate.sdpMid,
            sdpMLineIndex: event.candidate.sdpMLineIndex,
          };
      sendVoiceSignal("voice_ice", { candidate: candidate });
    });
    peer.addEventListener("track", function (event) {
      if (peer !== voicePeer) return;
      const audio = document.getElementById("remoteVoiceAudio");
      if (!audio) return;
      audio.srcObject = event.streams[0] || new MediaStream([event.track]);
      audio.play().then(function () {
        const resume = document.getElementById("resumeVoiceButton");
        if (resume) resume.hidden = true;
        updateVoiceConnectionStatus();
      }).catch(function () {
        const resume = document.getElementById("resumeVoiceButton");
        if (resume) resume.hidden = false;
        setVoiceStatus("تم الاتصال، لكن المتصفح ينتظر إذنك لتشغيل صوت المنافس.", "waiting");
      });
    });
    peer.addEventListener("connectionstatechange", function () {
      if (peer !== voicePeer || !voiceJoined) return;
      if (peer.connectionState === "connected") {
        updateVoiceConnectionStatus();
      } else if (peer.connectionState === "connecting") {
        setVoiceStatus("جاري إنشاء الاتصال الصوتي الآمن…", "connecting");
      } else if (peer.connectionState === "disconnected") {
        setVoiceStatus("انقطع صوت المنافس مؤقتًا؛ نحاول استعادته…", "waiting");
      } else if (peer.connectionState === "failed") {
        resetRemoteVoice("تعذر استعادة الصوت. نحاول بدء اتصال جديد…");
        announceVoiceReady(true);
      } else if (peer.connectionState === "closed") {
        setVoiceStatus("انتهى الاتصال الصوتي.", "idle");
      }
    });
    return peer;
  }

  function enqueueVoiceOperation(operation) {
    voiceOperation = voiceOperation.then(operation, operation).catch(function () {
      if (!voiceJoined) return;
      closeVoicePeer();
      setVoiceStatus("تعذر إنشاء الاتصال الصوتي. غادر الصوت ثم حاول الانضمام مجددًا.", "error");
    });
    return voiceOperation;
  }

  function validVoiceDescription(value, expectedType) {
    if (!value || typeof value !== "object" || value.type !== expectedType || typeof value.sdp !== "string") return null;
    if (!value.sdp || value.sdp.length > 12 * 1024) return null;
    return { type: value.type, sdp: value.sdp };
  }

  function validVoiceCandidate(value) {
    if (!value || typeof value !== "object" || typeof value.candidate !== "string" || value.candidate.length > 4096) return null;
    const candidate = { candidate: value.candidate };
    if (value.sdpMid === null || (typeof value.sdpMid === "string" && value.sdpMid.length <= 128)) candidate.sdpMid = value.sdpMid;
    if (value.sdpMLineIndex === null || (Number.isInteger(value.sdpMLineIndex) && value.sdpMLineIndex >= 0 && value.sdpMLineIndex <= 255)) candidate.sdpMLineIndex = value.sdpMLineIndex;
    if (typeof value.usernameFragment === "string" && value.usernameFragment.length <= 256) candidate.usernameFragment = value.usernameFragment;
    return candidate;
  }

  async function flushPendingVoiceCandidates(peer) {
    if (!peer.remoteDescription) return;
    const candidates = pendingVoiceCandidates.splice(0);
    for (const candidate of candidates) await peer.addIceCandidate(candidate);
  }

  async function createAndSendVoiceOffer() {
    const remoteUserID = voiceRemoteUserID || currentOpponentID();
    if (!voiceJoined || !voiceRemoteReady || !shouldInitiateVoice(remoteUserID)) return;
    const peer = createVoicePeer();
    if (peer.signalingState !== "stable") return;
    setVoiceStatus("جاري الاتصال بصوت المنافس…", "connecting");
    const offer = await peer.createOffer();
    await peer.setLocalDescription(offer);
    if (!voiceJoined || peer !== voicePeer || !peer.localDescription) return;
    sendVoiceSignal("voice_offer", {
      sdp: { type: peer.localDescription.type, sdp: peer.localDescription.sdp },
    });
  }

  async function acceptVoiceOffer(offer) {
    if (!voiceJoined) return;
    const peer = createVoicePeer();
    if (peer.signalingState !== "stable") await peer.setLocalDescription({ type: "rollback" });
    await peer.setRemoteDescription(offer);
    await flushPendingVoiceCandidates(peer);
    const answer = await peer.createAnswer();
    await peer.setLocalDescription(answer);
    if (!voiceJoined || peer !== voicePeer || !peer.localDescription) return;
    sendVoiceSignal("voice_answer", {
      sdp: { type: peer.localDescription.type, sdp: peer.localDescription.sdp },
    });
    setVoiceStatus("جاري إكمال الاتصال الصوتي…", "connecting");
  }

  async function acceptVoiceAnswer(answer) {
    if (!voiceJoined || !voicePeer || voicePeer.signalingState !== "have-local-offer") return;
    await voicePeer.setRemoteDescription(answer);
    await flushPendingVoiceCandidates(voicePeer);
  }

  async function acceptVoiceCandidate(candidate) {
    if (!voiceJoined) return;
    if (!voicePeer || !voicePeer.remoteDescription) {
      pendingVoiceCandidates.push(candidate);
      if (pendingVoiceCandidates.length > 64) pendingVoiceCandidates.shift();
      return;
    }
    await voicePeer.addIceCandidate(candidate);
  }

  function processVoiceSignal(update) {
    if (!currentAccount || !currentGame) {
      pendingVoiceSignals.push(update);
      if (pendingVoiceSignals.length > 64) pendingVoiceSignals.shift();
      return;
    }
    if (!isExpectedVoiceSender(update.fromUserId) || sameID(update.fromUserId, currentAccount.userId)) return;
    voiceRemoteUserID = update.fromUserId;
    const payload = update.payload && typeof update.payload === "object" ? update.payload : {};

    if (update.type === "voice_ready") {
      voiceRemoteReady = true;
      if (voiceJoined) {
        setVoiceStatus("المنافس جاهز للصوت. جاري إنشاء الاتصال…", "connecting");
        if (shouldInitiateVoice(voiceRemoteUserID)) enqueueVoiceOperation(createAndSendVoiceOffer);
        else sendVoiceSignal("voice_ready", {});
      }
      return;
    }
    if (update.type === "voice_leave") {
      resetRemoteVoice("غادر المنافس الصوت. سيظل ميكروفونك جاهزًا حتى مغادرتك.");
      return;
    }
    if (update.type === "voice_offer") {
      const offer = validVoiceDescription(payload.sdp, "offer");
      if (!offer) return;
      voiceRemoteReady = true;
      if (!voiceJoined) pendingVoiceOffer = offer;
      else enqueueVoiceOperation(function () { return acceptVoiceOffer(offer); });
      return;
    }
    if (update.type === "voice_answer") {
      const answer = validVoiceDescription(payload.sdp, "answer");
      if (answer && voiceJoined) enqueueVoiceOperation(function () { return acceptVoiceAnswer(answer); });
      return;
    }
    if (update.type === "voice_ice") {
      const candidate = validVoiceCandidate(payload.candidate);
      if (!candidate) return;
      if (!voiceJoined || !voicePeer || !voicePeer.remoteDescription) {
        pendingVoiceCandidates.push(candidate);
        if (pendingVoiceCandidates.length > 64) pendingVoiceCandidates.shift();
      } else {
        enqueueVoiceOperation(function () { return acceptVoiceCandidate(candidate); });
      }
    }
  }

  function flushPendingVoiceSignals() {
    if (!currentAccount || !currentGame || !pendingVoiceSignals.length) return;
    const signals = pendingVoiceSignals.splice(0);
    signals.forEach(processVoiceSignal);
  }

  function syncVoiceParticipants() {
    if (currentGame && (isBotMode() || !isDuelMode())) {
      if (voiceJoined || voiceJoining) leaveVoice({ notify: true });
      pendingVoiceSignals = [];
      renderVoiceControls();
      setVoiceStatus(isBotMode() ? "الدردشة الصوتية غير متاحة في مواجهة البوت." : "الدردشة الصوتية معطلة في الساحات الجماعية حاليًا.", "idle");
      return;
    }
    renderVoiceControls();
    if (!currentAccount || !currentGame) return;
    const opponentID = currentOpponentID();
    if (voiceRemoteUserID !== null && (opponentID === null || !sameID(voiceRemoteUserID, opponentID))) {
      voiceRemoteUserID = opponentID;
      announcedVoiceReadyFor = null;
      resetRemoteVoice("الميكروفون جاهز. بانتظار عودة المنافس إلى الساحة.");
    }
    flushPendingVoiceSignals();
    if (voiceJoined && opponentID !== null) announceVoiceReady(false);
  }

  function voiceJoinErrorMessage(error) {
    if (error && error.name === "NotAllowedError") return "لم يُسمح باستخدام الميكروفون. اسمح به من إعدادات المتصفح ثم حاول مجددًا.";
    if (error && error.name === "NotFoundError") return "لم يعثر المتصفح على ميكروفون متاح.";
    if (error && error.name === "NotReadableError") return "الميكروفون مستخدم في تطبيق آخر أو تعذر فتحه.";
    return "تعذر تشغيل الدردشة الصوتية على هذا الجهاز أو الاتصال.";
  }

  async function joinVoice() {
    if (!isDuelMode() || isBotMode()) {
      setVoiceStatus("الدردشة الصوتية متاحة حاليًا في مواجهات 1 ضد 1 فقط.", "idle");
      return;
    }
    if (voiceJoined || voiceJoining || stopped) return;
    if (!window.RTCPeerConnection || !navigator.mediaDevices || typeof navigator.mediaDevices.getUserMedia !== "function") {
      setVoiceStatus("هذا المتصفح لا يدعم الدردشة الصوتية الآمنة.", "error");
      return;
    }
    voiceJoining = true;
    const attempt = ++voiceJoinAttempt;
    setVoiceStatus("بانتظار إذن استخدام الميكروفون…", "requesting");
    renderVoiceControls();
    let stream = null;
    try {
      stream = await navigator.mediaDevices.getUserMedia({
        audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true },
        video: false,
      });
      if (attempt !== voiceJoinAttempt || stopped) {
        stream.getTracks().forEach(function (track) { track.stop(); });
        return;
      }
      localVoiceStream = stream;
      voiceJoined = true;
      voiceMuted = false;
      voiceRemoteUserID = currentOpponentID();
      announcedVoiceReadyFor = null;
      renderVoiceControls();
      announceVoiceReady(true);
      setVoiceStatus(voiceRemoteReady ? "المنافس جاهز. جاري إنشاء الاتصال الصوتي…" : "الميكروفون جاهز. بانتظار انضمام المنافس للصوت.", voiceRemoteReady ? "connecting" : "waiting");
      if (pendingVoiceOffer) {
        const offer = pendingVoiceOffer;
        pendingVoiceOffer = null;
        enqueueVoiceOperation(function () { return acceptVoiceOffer(offer); });
      } else if (voiceRemoteReady && shouldInitiateVoice(voiceRemoteUserID)) {
        enqueueVoiceOperation(createAndSendVoiceOffer);
      }
    } catch (error) {
      if (stream) stream.getTracks().forEach(function (track) { track.stop(); });
      stopLocalVoiceStream();
      voiceJoined = false;
      setVoiceStatus(voiceJoinErrorMessage(error), "error");
    } finally {
      if (attempt === voiceJoinAttempt) voiceJoining = false;
      renderVoiceControls();
    }
  }

  function toggleVoiceMute() {
    if (!voiceJoined || !localVoiceStream) return;
    voiceMuted = !voiceMuted;
    localVoiceStream.getAudioTracks().forEach(function (track) { track.enabled = !voiceMuted; });
    renderVoiceControls();
    if (voicePeer && voicePeer.connectionState === "connected") updateVoiceConnectionStatus();
    else setVoiceStatus(voiceMuted ? "الميكروفون مكتوم أثناء انتظار المنافس." : "الميكروفون يعمل. بانتظار اتصال المنافس.", voiceMuted ? "muted" : "waiting");
  }

  function resumeRemoteVoice() {
    const audio = document.getElementById("remoteVoiceAudio");
    if (!audio || !audio.srcObject) return;
    audio.play().then(function () {
      const resume = document.getElementById("resumeVoiceButton");
      if (resume) resume.hidden = true;
      updateVoiceConnectionStatus();
    }).catch(function () {
      setVoiceStatus("تعذر تشغيل صوت المنافس. تحقق من إعدادات الصوت في المتصفح.", "error");
    });
  }

  function leaveVoice(options) {
    const notifyRemote = !options || options.notify !== false;
    if (notifyRemote && (voiceJoined || voiceJoining)) sendVoiceSignal("voice_leave", {});
    voiceJoinAttempt += 1;
    voiceJoining = false;
    voiceJoined = false;
    voiceMuted = false;
    voiceRemoteReady = false;
    voiceRemoteUserID = null;
    announcedVoiceReadyFor = null;
    pendingVoiceOffer = null;
    pendingVoiceCandidates = [];
    stopLocalVoiceStream();
    closeVoicePeer();
    renderVoiceControls();
    setVoiceStatus("الصوت غير متصل.", "idle");
  }

  function matchPlayer(userID) {
    return matchPlayers().find(function (player) { return sameID(player.userId, userID); });
  }

  function playerViews() {
    const known = new Set();
    const views = joinedUsers().map(function (user) {
      const player = matchPlayer(user.id);
      known.add(String(user.id));
      return {
        userId: user.id,
        fullName: user.fullName || "اللاعب #" + user.id,
        isBot: Boolean(user.isBot || (player && player.isBot)),
        botStrategy: user.botStrategy || (player && player.botStrategy) || currentGame.botStrategy || "",
        team: Number(player && player.team !== undefined ? player.team : user.team) || 0,
        score: Number(player && player.score) || 0,
        deckReady: Boolean(player && player.deckReady),
      };
    });
    matchPlayers().forEach(function (player) {
      if (known.has(String(player.userId))) return;
      views.push({
        userId: player.userId,
        fullName: playerName(player.userId),
        isBot: Boolean(player.isBot),
        botStrategy: player.botStrategy || "",
        team: Number(player.team) || 0,
        score: Number(player.score) || 0,
        deckReady: Boolean(player.deckReady),
      });
    });
    return views;
  }

  function rankedPlayerViews() {
    const result = playerViews().slice().sort(function (left, right) {
      if (right.score !== left.score) return right.score - left.score;
      return left.fullName.localeCompare(right.fullName, "ar");
    });
    let rank = 0;
    let previousScore = null;
    result.forEach(function (player, index) {
      if (previousScore === null || player.score !== previousScore) rank = index + 1;
      player.rank = rank;
      previousScore = player.score;
    });
    return result;
  }

  function winnerIDs() {
    if (!currentMatch) return [];
    if (Array.isArray(currentMatch.winnerIds)) return currentMatch.winnerIds.map(String);
    return currentMatch.winnerId ? [String(currentMatch.winnerId)] : [];
  }

  function championID() {
    if (!currentMatch) return null;
    if (currentMatch.winnerId) return String(currentMatch.winnerId);
    const winners = winnerIDs();
    return winners.length === 1 ? winners[0] : null;
  }

  function tieBreakIDs() {
    if (!currentMatch) return [];
    if (currentMatch.tieBreak && Array.isArray(currentMatch.tieBreak.contenderIds)) return currentMatch.tieBreak.contenderIds.map(String);
    if (Array.isArray(currentMatch.tieBreakContenderIds)) return currentMatch.tieBreakContenderIds.map(String);
    const turn = currentMatch.currentTurn;
    return turn && turn.kind === "tie_break" && Array.isArray(turn.eligibleUserIds) ? turn.eligibleUserIds.map(String) : [];
  }

  function isTieBreakTurn(turn) {
    return Boolean(turn && (turn.kind === "tie_break" || Number(turn.tieBreakRound) > 0));
  }

  function tieBreakRound() {
    if (!currentMatch) return 0;
    const nested = currentMatch.tieBreak;
    if (nested && Number(nested.round) > 0) return Number(nested.round);
    return Number(currentMatch.tieBreakRound) || Number(currentMatch.currentTurn && currentMatch.currentTurn.tieBreakRound) || 0;
  }

  function tieBreakScope() {
    const nested = currentMatch && currentMatch.tieBreak;
    const phase = nested && nested.phase ? nested.phase : currentMatch && currentMatch.tieBreakScope;
    return phase === "teams" || phase === "team" ? "team" : "champion";
  }

  function renderParticipants() {
    const list = document.getElementById("battle_Joined_Users_container");
    if (!list) return;
    list.replaceChildren();
    playerViews().forEach(function (player) {
      const row = document.createElement("div");
      row.className = "qb-participant-row" + (sameID(player.userId, currentAccount.userId) ? " is-current" : "");
      if (player.team > 0) row.dataset.team = String(player.team);

      const avatar = document.createElement("span");
      avatar.className = "qb-participant-avatar" + (player.isBot ? " is-bot" : "");
      avatar.textContent = player.isBot ? "◆" : initialForName(player.fullName);
      avatar.setAttribute("aria-hidden", "true");
      const identity = document.createElement("span");
      identity.className = "qb-participant-identity";
      const name = document.createElement("strong");
      name.textContent = player.fullName;
      const meta = document.createElement("small");
      const ownerSuffix = currentGame.owner && sameID(currentGame.owner.id, player.userId) ? " · المالك" : "";
      meta.textContent = player.isBot
        ? "بوت المعرفة · " + botStrategyLabel(player.botStrategy)
        : "#" + player.userId + (player.team > 0 ? " · " + teamLabel(player.team) : "") + ownerSuffix;
      identity.append(name, meta);

      const facts = document.createElement("span");
      facts.className = "qb-participant-facts";
      const score = document.createElement("strong");
      score.textContent = player.score + " نقطة";
      const ready = document.createElement("small");
      ready.className = "qb-player-state";
      ready.dataset.state = player.deckReady ? "ready" : "waiting";
      ready.textContent = player.deckReady ? "جاهز" : "غير جاهز";
      facts.append(score, ready);
      row.append(avatar, identity, facts);
      list.appendChild(row);
    });
  }

  function renderTeamScoreboard(views) {
    const container = document.getElementById("teamScoreboard");
    if (!container) return;
    container.replaceChildren();
    container.hidden = !isTeamMode();
    if (!isTeamMode()) return;

    const totals = new Map();
    views.forEach(function (player) {
      if (player.team > 0) totals.set(player.team, (totals.get(player.team) || 0) + player.score);
    });
    if (currentMatch && Array.isArray(currentMatch.teamScores)) {
      currentMatch.teamScores.forEach(function (team) { totals.set(Number(team.team), Number(team.score) || 0); });
    } else if (currentMatch && Array.isArray(currentMatch.teams)) {
      currentMatch.teams.forEach(function (team) { totals.set(Number(team.id), Number(team.score) || 0); });
    }
    Array.from(totals.entries()).sort(function (left, right) { return left[0] - right[0]; }).forEach(function (entry) {
      const team = document.createElement("div");
      team.className = "qb-team-score";
      team.dataset.team = String(entry[0]);
      const label = document.createElement("span");
      label.textContent = teamLabel(entry[0]);
      const score = document.createElement("strong");
      score.textContent = entry[1] + " نقطة";
      team.append(label, score);
      container.appendChild(team);
    });
  }

  function renderLeaderboard() {
    const list = document.getElementById("scoreboardPlayers");
    const views = rankedPlayerViews();
    if (!list) return;
    list.replaceChildren();
    const winners = new Set(winnerIDs());
    const contenders = new Set(tieBreakIDs());

    views.forEach(function (player) {
      const row = document.createElement("li");
      row.className = "qb-scoreboard-player";
      if (sameID(player.userId, currentAccount.userId)) row.classList.add("is-current");
      if (winners.has(String(player.userId))) row.classList.add("is-winner");
      if (sameID(player.userId, championID())) row.classList.add("is-champion");
      if (contenders.has(String(player.userId))) row.classList.add("is-contender");
      if (player.team > 0) row.dataset.team = String(player.team);

      const rank = document.createElement("span");
      rank.className = "qb-scoreboard-player__rank";
      rank.textContent = String(player.rank);
      rank.setAttribute("aria-label", "المركز " + player.rank);
      const avatar = document.createElement("span");
      avatar.className = "qb-scoreboard-player__avatar" + (player.isBot ? " is-bot" : "");
      avatar.textContent = player.isBot ? "◆" : initialForName(player.fullName);
      avatar.setAttribute("aria-hidden", "true");
      const copy = document.createElement("span");
      copy.className = "qb-scoreboard-player__copy";
      const name = document.createElement("strong");
      name.textContent = player.fullName;
      const meta = document.createElement("small");
      meta.textContent = sameID(player.userId, championID())
        ? "بطل الساحة النهائي"
        : (player.isBot ? "بوت " + botStrategyLabel(player.botStrategy) : (player.team > 0 ? teamLabel(player.team) : (player.deckReady ? "جاهز" : "بانتظار الجاهزية")));
      copy.append(name, meta);
      const score = document.createElement("strong");
      score.className = "qb-scoreboard-player__score";
      score.textContent = player.score + " نقطة";
      row.append(rank, avatar, copy, score);
      list.appendChild(row);
    });

    renderTeamScoreboard(views);
    const readyCount = views.filter(function (player) { return player.deckReady; }).length;
    setText("readySummary", currentMatch ? readyCount + " من " + views.length + " جاهزون" : views.length + " في الساحة");
    const leader = views[0];
    const champion = views.find(function (player) { return sameID(player.userId, championID()); });
    if (currentMatch && currentMatch.status === "forfeited") {
      setText("playerScore", "لا يوجد فائز");
      setText("opponentScore", "انتهت بالانسحاب");
    } else if (currentMatch && currentMatch.status === "completed" && champion) {
      setText("playerScore", champion.fullName);
      setText("opponentScore", champion.score + " نقطة · البطل");
    } else {
      setText("playerScore", leader ? leader.fullName : "—");
      setText("opponentScore", leader ? leader.score + " نقطة" : "—");
    }
  }

  function renderTieBreak() {
    const banner = document.getElementById("tieBreakBanner");
    if (!banner) return;
    const turn = currentMatch && currentMatch.currentTurn;
    const nested = currentMatch && currentMatch.tieBreak;
    const round = tieBreakRound();
    const terminal = currentMatch && ["completed", "forfeited"].includes(currentMatch.status);
    const active = Boolean(currentMatch && !terminal && (currentMatch.status === "tie_break" || (nested && nested.active) || isTieBreakTurn(turn)));
    banner.hidden = !active;
    document.body.classList.toggle("qb-is-tiebreak", active);
    if (!active) return;

    setText("tieBreakRound", String(round || 1));
    const contenderIDs = tieBreakIDs();
    const scope = tieBreakScope() === "team" ? "الفرق المتعادلة" : "اللاعبين المتعادلين";
    setText("tieBreakCopy", "يتنافس " + scope + " في أسئلة إضافية حتى يتبقى فائز واحد نهائي.");
    const container = document.getElementById("tieBreakParticipants");
    if (!container) return;
    container.replaceChildren();
    contenderIDs.forEach(function (userID) {
      const chip = document.createElement("span");
      chip.className = "qb-tiebreak-player";
      const avatar = document.createElement("span");
      const contender = playerViews().find(function (player) { return sameID(player.userId, userID); });
      avatar.textContent = contender && contender.isBot ? "◆" : initialForName(playerName(userID));
      if (contender && contender.isBot) avatar.classList.add("is-bot");
      avatar.setAttribute("aria-hidden", "true");
      const name = document.createElement("strong");
      name.textContent = playerName(userID);
      chip.append(avatar, name);
      container.appendChild(chip);
    });
  }

  function renderArenaReadiness() {
    const config = modeConfig();
    const views = playerViews();
    setText("arenaCapacity", views.length + " / " + config.maxPlayers + (isBotMode() ? " مقعدين" : " لاعبين"));
    const container = document.getElementById("opponentDeck");
    if (container) {
      container.replaceChildren();
      views.forEach(function (player) {
        const chip = document.createElement("span");
        chip.className = "qb-deck-status-chip";
        chip.dataset.state = player.deckReady ? "ready" : "waiting";
        if (player.team > 0) chip.dataset.team = String(player.team);
        const avatar = document.createElement("span");
        avatar.textContent = player.isBot ? "◆" : initialForName(player.fullName);
        if (player.isBot) avatar.classList.add("is-bot");
        avatar.setAttribute("aria-hidden", "true");
        const copy = document.createElement("span");
        const name = document.createElement("strong");
        name.textContent = player.fullName;
        const state = document.createElement("small");
        state.textContent = player.isBot
          ? (player.deckReady ? "جهّز 5 بطاقات" : "يجهّز بطاقاته")
          : (player.deckReady ? "ثبت 5 بطاقات" : "لم يثبت بطاقاته");
        copy.append(name, state);
        chip.append(avatar, copy);
        container.appendChild(chip);
      });
    }
    const own = matchPlayer(currentAccount.userId);
    const ownBadge = document.getElementById("ownReadyBadge");
    if (ownBadge) {
      ownBadge.dataset.state = own && own.deckReady ? "ready" : "waiting";
      ownBadge.textContent = own && own.deckReady ? "جاهز" : "غير جاهز";
    }
  }

  function renderBotActivity() {
    const badge = document.getElementById("botBattleBadge");
    const activity = document.getElementById("botActivity");
    const bot = playerViews().find(function (player) { return player.isBot; });
    const botMode = isBotMode();
    if (badge) {
      badge.hidden = !botMode;
      badge.textContent = botMode ? "ضد البوت · " + botStrategyLabel((bot && bot.botStrategy) || (currentGame && currentGame.botStrategy)) : "";
    }
    if (!activity) return;
    if (!botMode || (currentMatch && ["completed", "forfeited"].includes(currentMatch.status))) {
      activity.hidden = true;
      activity.textContent = "";
      activity.dataset.state = "idle";
      return;
    }

    let message = "بوت المعرفة في مقعده وينتظر تجهيز المواجهة.";
    let state = "ready";
    if (currentMatch && currentMatch.status === "collecting_decks") {
      message = bot && bot.deckReady ? "جهّز بوت المعرفة بطاقاته؛ ثبّت مجموعتك عندما تصبح جاهزًا." : "بوت المعرفة يجهّز بطاقاته…";
      state = bot && bot.deckReady ? "ready" : "thinking";
    } else if (currentMatch && ["active", "tie_break"].includes(currentMatch.status)) {
      const turn = currentMatch.currentTurn;
      const eligible = Boolean(turn && bot && Array.isArray(turn.eligibleUserIds) && turn.eligibleUserIds.some(function (id) { return sameID(id, bot.userId); }));
      const answered = Boolean(turn && bot && Array.isArray(turn.answeredUserIds) && turn.answeredUserIds.some(function (id) { return sameID(id, bot.userId); }));
      if (turn && turn.status === "resolved") {
        message = "انتهى الدور وكُشفت إجابة بوت المعرفة.";
        state = "answered";
      } else if (eligible && answered) {
        message = "سجّل بوت المعرفة إجابته لدى الخادم.";
        state = "answered";
      } else if (eligible) {
        message = "بوت المعرفة يفكر…";
        state = "thinking";
      } else {
        message = "بوت المعرفة يتابع هذا الدور.";
        state = "watching";
      }
    }
    activity.hidden = false;
    activity.dataset.state = state;
    if (activity.textContent !== message) activity.textContent = message;
  }

  function normalizeReward(match) {
    const receipt = match && match.reward && typeof match.reward === "object" ? match.reward : null;
    const rawStatus = receipt && typeof receipt.status === "string" ? receipt.status.toLowerCase() : "";
    const allowedStatuses = new Set(["granted", "capped", "ineligible", "pending"]);
    let status = allowedStatuses.has(rawStatus) ? rawStatus : "";
    if (!status) {
      if (match && match.status === "forfeited") status = "ineligible";
      else if (match && match.rewardsSettled === false) status = "pending";
      else status = "granted";
    }
    const rawCoins = receipt && receipt.coinsGranted !== undefined ? receipt.coinsGranted : match && match.rewardCoins;
    const parsedCoins = Number(rawCoins);
    return {
      status: status,
      coinsGranted: Number.isFinite(parsedCoins) ? Math.max(0, Math.trunc(parsedCoins)) : 0,
      card: receipt && receipt.card && typeof receipt.card === "object" ? receipt.card : null,
      reason: receipt && typeof receipt.reason === "string" ? receipt.reason : "",
      legacy: !receipt,
    };
  }

  function rewardReasonLabel(value) {
    const reasons = {
      bot_daily_cap: "بلغت الحد اليومي لمكافآت مواجهات البوت. يمكنك الاستمرار في اللعب دون مكافأة.",
      pvp_daily_cap: "بلغت الحد اليومي لمكافآت مواجهات اللاعبين. يمكنك الاستمرار في اللعب دون مكافأة.",
      daily_cap: "بلغت الحد اليومي لمكافآت مواجهات البوت. يمكنك الاستمرار في اللعب دون مكافأة.",
      capped: "بلغت الحد المسموح لمكافآت مواجهات البوت.",
      insufficient_participation: "لم تسجل عدد الإجابات المطلوب للحصول على المكافأة.",
      forfeit: "لا تُمنح مكافأة عند إنهاء المواجهة قبل اكتمالها.",
      loss: "هذه المواجهة لا تمنح مكافأة عند الخسارة.",
      draw: "لم تُمنح مكافأة قبل حسم فائز نهائي.",
    };
    if (!value) return "";
    if (reasons[value]) return reasons[value];
    return /[\u0600-\u06ff\s]/.test(value) ? value : "لم تتحقق شروط المكافأة لهذه المواجهة.";
  }

  function renderResultScores() {
    const container = document.getElementById("resultScores");
    if (!container) return;
    container.replaceChildren();
    rankedPlayerViews().forEach(function (player) {
      const row = document.createElement("div");
      row.className = "qb-match-result__score" + (player.isBot ? " is-bot" : "");
      const identity = document.createElement("span");
      const avatar = document.createElement("span");
      avatar.className = "qb-match-result__avatar" + (player.isBot ? " is-bot" : "");
      avatar.textContent = player.isBot ? "◆" : initialForName(player.fullName);
      avatar.setAttribute("aria-hidden", "true");
      const name = document.createElement("strong");
      name.textContent = player.fullName;
      identity.append(avatar, name);
      const score = document.createElement("strong");
      score.textContent = player.score + " نقطة";
      row.append(identity, score);
      container.appendChild(row);
    });
  }

  function renderRewardCard(card) {
    const container = document.getElementById("rewardCard");
    if (!container) return;
    container.hidden = !card;
    if (!card) return;
    const category = card.category || "general-knowledge";
    const rarity = ["common", "rare", "epic", "legendary"].includes(card.rarity) ? card.rarity : "common";
    const art = document.getElementById("rewardCardArt");
    if (art && window.QuizBattleCardVisuals) QuizBattleCardVisuals.applyArt(art, category, { eager: true });
    ["common", "rare", "epic", "legendary"].forEach(function (value) { container.classList.remove("rarity--" + value); });
    container.classList.add("rarity--" + rarity);
    setText("rewardCardRarity", rarityLabel(rarity));
    setText("rewardCardTitle", card.prompt || card.title || "بطاقة " + categoryLabel(category));
    const meta = [categoryLabel(category), difficultyLabel(card.difficulty), card.power ? "القوة " + card.power : ""].filter(Boolean);
    setText("rewardCardMeta", meta.join(" · "));
  }

  function renderMatchResult(message, forfeited) {
    const question = document.getElementById("questionCard");
    const result = document.getElementById("matchResult");
    clearInterval(clockTimer);
    clockTimer = null;
    if (question) question.hidden = true;
    if (!result) return;
    result.hidden = false;
    result.dataset.outcome = forfeited ? "forfeited" : (sameID(championID(), currentAccount.userId) ? "won" : "lost");
    setText("matchResultEyebrow", forfeited ? "انتهت بالانسحاب" : "انتهت المواجهة");
    setText("matchResultTitle", forfeited ? "لم تكتمل المواجهة" : (sameID(championID(), currentAccount.userId) ? "أنت بطل الساحة" : "النتيجة النهائية"));
    setText("matchResultSummary", message);
    renderResultScores();

    const reward = normalizeReward(currentMatch);
    const formattedCoins = new Intl.NumberFormat("ar-EG").format(reward.coinsGranted);
    setText("rewardCoinsGranted", formattedCoins + " عملة");
    const statusCopy = {
      granted: reward.coinsGranted > 0 || reward.card ? "تمت إضافة مكافأتك إلى حسابك." : "اكتملت تسوية المواجهة دون مكافأة إضافية.",
      capped: "اكتملت المواجهة، لكن لم تُمنح مكافأة بسبب الحد الحالي.",
      ineligible: "انتهت المواجهة دون مكافأة.",
      pending: "يؤكد الخادم مكافأتك الآن…",
    };
    setText("rewardStatus", statusCopy[reward.status]);
    const reason = document.getElementById("rewardReason");
    const reasonCopy = rewardReasonLabel(reward.reason);
    if (reason) {
      reason.hidden = !reasonCopy;
      reason.textContent = reasonCopy;
    }
    renderRewardCard(reward.card);

    const matchKey = String(currentMatch.id || currentMatch.gameId || battleID) + ":" + reward.status + ":" + reward.coinsGranted + ":" + String(reward.card && reward.card.id || "");
    if (reward.status === "granted" && collectionRefreshKey !== matchKey) {
      collectionRefreshKey = matchKey;
      loadCollection();
    }
    const focusKey = String(currentMatch.id || currentMatch.gameId || battleID) + ":" + currentMatch.status;
    if (lastResultFocusKey !== focusKey) {
      lastResultFocusKey = focusKey;
      window.requestAnimationFrame(function () {
        if (!result.hidden) result.focus();
      });
    }
  }

  function showQuestionSurface() {
    const question = document.getElementById("questionCard");
    const result = document.getElementById("matchResult");
    if (question) question.hidden = false;
    if (result) result.hidden = true;
  }

  function renderExitButton() {
    const exit = document.getElementById("exitBattleButton");
    if (!exit) return;
    exit.hidden = false;
    const inProgress = currentMatch && ["collecting_decks", "active", "tie_break"].includes(currentMatch.status);
    const owner = currentGame && currentAccount && sameID(currentGame.owner.id, currentAccount.userId);
    const botMode = isBotMode();
    const ownerOnlyCancellation = inProgress && !isDuelMode();
    if (ownerOnlyCancellation && !owner) {
      exit.disabled = true;
      exit.textContent = "الإلغاء متاح للمالك فقط";
      exit.title = "حمايةً لبقية المشاركين، لا يستطيع إلغاء الساحة الجماعية إلا مالكها";
      return;
    }
    exit.disabled = false;
    exit.textContent = inProgress
      ? (botMode ? "إنهاء مواجهة البوت" : (ownerOnlyCancellation ? "إلغاء المنافسة" : "الانسحاب من المنافسة"))
      : "مغادرة الساحة";
    exit.title = inProgress
      ? (botMode ? "ينهي المواجهة دون مكافأة ويحرر بطاقاتك" : (ownerOnlyCancellation ? "ينهي الساحة الجماعية ويحرر بطاقات جميع المشاركين" : "يسجل انسحابك ويحرر بطاقاتك وفق قواعد الساحة"))
      : "مغادرة الساحة";
  }

  function renderGame() {
    if (!currentGame || !currentAccount) return;
    const config = modeConfig();
    const botMode = isBotMode();
    document.title = botMode ? "QuizBattle — مواجهة بوت المعرفة" : "QuizBattle — ساحة " + currentGame.owner.fullName;
    setText("battleTitle", botMode ? currentGame.owner.fullName + " ضد حارس المعرفة" : "ساحة " + currentGame.owner.fullName + " · " + config.label);
    setText("arenaModeBadge", botMode ? "ضد البوت · مواجهة خاصة" : config.label + " · حتى " + config.maxPlayers + " لاعبين");
    setText("battleLead", botMode
      ? "اختر خمس بطاقات، ثم ابدأ مواجهة خاصة ضد بوت المعرفة. قرارات البوت والنتيجة والمكافآت يؤكدها الخادم."
      : (gameMode() === "open"
        ? "ساحة مفتوحة تبدأ من لاعبين. يجهز المالك الساحة ثم تبدأ عندما يثبت جميع الموجودين خمس بطاقات."
        : "ساحة " + config.label + ". بعد اكتمال " + config.maxPlayers + " لاعبين يجهز المالك الساحة، ثم تبدأ عند جاهزية الجميع."));
    renderParticipants();
    renderLeaderboard();
    renderArenaReadiness();
    renderBotActivity();
    syncVoiceParticipants();
    renderExitButton();
    const loader = document.getElementById("battlepage");
    if (loader) loader.hidden = true;
  }

  function renderMatch() {
    if (!currentAccount || !currentCollection) return;
    renderExitButton();
    const own = matchPlayer(currentAccount.userId);
    setText("matchConnectionStatus", battleSocket && battleSocket.readyState === WebSocket.OPEN ? "متصل" : "إعادة اتصال");
    renderParticipants();
    renderLeaderboard();
    renderArenaReadiness();
    renderTieBreak();
    renderBotActivity();

    const config = modeConfig();
    const joinedPlayers = joinedUsers().length;
    if (!currentMatch) {
      showQuestionSurface();
      renderWaitingDecks();
      renderPrepareButton();
      renderStartButton();
      setText("roundIndicator", "قبل التجهيز");
      setText("turnIndicator", joinedPlayers + " من " + config.maxPlayers + " لاعبين");
      const enough = joinedPlayers >= config.minPlayers;
      const fixedModeFull = gameMode() === "open" || joinedPlayers === config.maxPlayers;
      const waitingMessage = !enough
        ? "بانتظار انضمام " + (config.minPlayers - joinedPlayers) + " لاعب إضافي على الأقل."
        : (!fixedModeFull
          ? "بانتظار اكتمال مقاعد الساحة قبل أن يجهزها المالك."
          : (isOwner() ? (isBotMode() ? "بوت المعرفة جاهز. جهّز المواجهة لاختيار بطاقاتك." : "اكتملت شروط التجهيز. اضغط تجهيز الساحة لفتح اختيار البطاقات.") : "اكتملت شروط التجهيز. بانتظار مالك الساحة."));
      renderWaitingQuestion(waitingMessage);
      renderLog([waitingMessage, "بعد التجهيز يثبت كل لاعب خمس بطاقات ليصبح جاهزًا."]);
      return;
    }

    renderPrepareButton();
    renderStartButton();

    if (currentMatch.status === "collecting_decks") {
      showQuestionSurface();
      renderDecks(own);
      setText("roundIndicator", "التجهيز");
      setText("turnIndicator", (own && own.deckReady ? "مجموعتك جاهزة" : "اختر مجموعتك"));
      const readyCount = matchPlayers().filter(function (player) { return player.deckReady; }).length;
      renderWaitingQuestion(own && own.deckReady
        ? (isBotMode() ? "تم حجز بطاقاتك بأمان. ابدأ المواجهة عندما يؤكد الخادم جاهزية البوت." : "تم حجز بطاقاتك بأمان. بانتظار جاهزية بقية اللاعبين ثم يبدأ المالك.")
        : "اختر خمس بطاقات متاحة وثبّتها لتعلن جاهزيتك.");
      renderLog([
        own && own.deckReady ? "تم تثبيت مجموعتك وأصبحت جاهزًا." : "مجموعتك لم تُثبت بعد.",
        readyCount + " من " + matchPlayers().length + " لاعبين جاهزون.",
      ]);
      return;
    }

    const deckSetup = document.getElementById("deckSetup");
    if (deckSetup) deckSetup.hidden = true;
    removeCommitButton();

    if (["completed", "forfeited"].includes(currentMatch.status)) {
      setText("roundIndicator", "انتهت");
      const forfeited = currentMatch.status === "forfeited";
      const winners = winnerIDs();
      const champion = championID();
      const viewerChampion = champion !== null && sameID(champion, currentAccount.userId);
      const viewerOnWinningTeam = winners.includes(String(currentAccount.userId));
      setText("turnIndicator", forfeited ? "انسحاب" : (viewerChampion ? "بطل الساحة" : (viewerOnWinningTeam ? "فريق فائز" : "انتهت")));
      let message = "";
      if (forfeited) {
        message = "انتهت المنافسة بالانسحاب، وتم تحرير البطاقات وفق قواعد الساحة.";
      } else if (Number(currentMatch.winnerTeam) > 0) {
        if (viewerChampion) message = "أحسنت! فاز " + teamLabel(currentMatch.winnerTeam) + " وأنت بطل الساحة النهائي.";
        else if (viewerOnWinningTeam) message = "أحسنت! فاز فريقك، وبطل الساحة النهائي هو " + playerName(champion) + ".";
        else message = "فاز " + teamLabel(currentMatch.winnerTeam) + "، وبطل الساحة النهائي هو " + playerName(champion) + ".";
      } else {
        message = viewerChampion ? "أحسنت! أنت بطل الساحة النهائي." : "حُسمت الساحة لصالح " + playerName(champion) + ".";
      }
      renderMatchResult(message, forfeited);
      renderLog([message, "تم إطلاق كل البطاقات المحجوزة بأمان."]);
      return;
    }

    showQuestionSurface();
    const turn = currentMatch.currentTurn;
    if (!turn) {
      renderWaitingQuestion("يحضّر الخادم الدور التالي…");
      return;
    }
    const tieBreakActive = currentMatch.status === "tie_break" || isTieBreakTurn(turn) || Boolean(currentMatch.tieBreak && currentMatch.tieBreak.active);
    setText("roundIndicator", tieBreakActive ? "حسم " + (tieBreakRound() || 1) : turn.round + " / 5");
    setText("turnIndicator", tieBreakActive ? "سؤال الحسم" : turn.number + " / " + (currentMatch.totalTurns || "—"));
    renderQuestion(turn);
    const eligibleCount = Array.isArray(turn.eligibleUserIds) ? turn.eligibleUserIds.length : matchPlayers().length;
    const answeredCount = Array.isArray(turn.answeredUserIds) ? turn.answeredUserIds.length : 0;
    renderLog([
      (tieBreakActive ? "سؤال الحسم" : "الدور " + turn.number) + " من بطاقة " + playerName(turn.cardOwnerId) + ".",
      answeredCount + " من " + eligibleCount + " سجلوا إجابتهم.",
      turn.status === "resolved" ? "أغلق الدور وكُشفت النتيجة." : "الدور نشط حتى الموعد الخادمي.",
    ]);
  }

  function renderWaitingDecks() {
    const playerDeck = document.getElementById("playerDeck");
    const deckSetup = document.getElementById("deckSetup");
    if (!playerDeck) return;
    if (deckSetup) deckSetup.hidden = true;
    playerDeck.classList.remove("qb-card-picker");
    playerDeck.replaceChildren();
    for (let index = 0; index < 5; index += 1) {
      playerDeck.appendChild(deckSlot("تُفتح بعد تجهيز الساحة", false));
    }
    removeCommitButton();
    renderArenaReadiness();
  }

  function renderDecks(own) {
    const playerDeck = document.getElementById("playerDeck");
    const deckSetup = document.getElementById("deckSetup");
    if (!playerDeck) return;
    if (deckSetup) deckSetup.hidden = false;
    playerDeck.replaceChildren();

    const ownDeckIDs = own && own.deckCardIds ? own.deckCardIds.map(String) : [];
    const serverDeckKey = ownDeckIDs.join("|");
    if (serverDeckKey !== selectedDeckSnapshotKey) {
      selectedCards.clear();
      ownDeckIDs.forEach(function (id) { selectedCards.add(id); });
      selectedDeckSnapshotKey = serverDeckKey;
    }
    const choosing = !currentMatch || currentMatch.status === "collecting_decks";
    renderDeckAssist(choosing);
    if (choosing) {
      playerDeck.classList.add("qb-card-picker");
      currentCollection.cards.forEach(function (card) {
        const cardID = String(card.id);
        const selectable = window.QuizBattleDeckRanking
          ? QuizBattleDeckRanking.isEligible(card, currentMatch && currentMatch.id)
          : card.status === "available" || ownDeckIDs.includes(cardID);
        const selectionFull = selectedCards.size >= 5 && !selectedCards.has(cardID);
        const button = document.createElement("button");
        button.type = "button";
        button.className = "qb-deck-slot qb-deck-choice qb-mini-card rarity--" + card.rarity;
        button.dataset.cardId = cardID;
        button.disabled = !selectable || selectionFull;
        button.classList.toggle("is-selected", selectedCards.has(cardID));
        button.setAttribute("aria-pressed", selectedCards.has(cardID) ? "true" : "false");
        button.setAttribute("aria-label", card.prompt + "، " + categoryLabel(card.category) + "، " + rarityLabel(card.rarity) + (selectedCards.has(cardID) ? "، محددة" : ""));
        button.title = selectionFull ? "اكتملت خمس بطاقات؛ أزل بطاقة أولًا" : categoryLabel(card.category) + " · " + rarityLabel(card.rarity) + " · #" + card.id;
        const art = window.QuizBattleCardVisuals
          ? QuizBattleCardVisuals.createArt(card.category)
          : document.createElement("span");
        art.classList.add("qb-mini-card__art");
        const copy = document.createElement("span");
        copy.className = "qb-mini-card__copy";
        copy.textContent = card.prompt;
        const meta = document.createElement("small");
        meta.className = "qb-mini-card__meta";
        meta.textContent = categoryLabel(card.category) + " · " + rarityLabel(card.rarity);
        button.append(art, copy, meta);
        button.addEventListener("click", function () {
          if (selectedCards.has(cardID)) selectedCards.delete(cardID);
          else if (selectedCards.size < 5) selectedCards.add(cardID);
          deckAssistMessage = "تم تحديث اختيارك يدويًا. يمكنك الاستمرار في التعديل ثم تثبيت المجموعة.";
          renderDecks(matchPlayer(currentAccount.userId));
          updateCommitButton();
          const refreshed = playerDeck.querySelector('[data-card-id="' + cardID + '"]');
          if (refreshed) refreshed.focus();
        });
        playerDeck.appendChild(button);
      });
      ensureCommitButton();
    } else {
      playerDeck.classList.remove("qb-card-picker");
      ownDeckIDs.forEach(function (cardID) {
        const card = currentCollection.cards.find(function (candidate) { return sameID(candidate.id, cardID); });
        playerDeck.appendChild(deckSlot(card ? card.prompt : "بطاقة محجوزة", false, card));
      });
      removeCommitButton();
    }
    if (!playerDeck.children.length) {
      for (let index = 0; index < 5; index += 1) playerDeck.appendChild(deckSlot("بطاقة", false));
    }
    renderArenaReadiness();
  }

  function eligibleDeckCards() {
    if (!window.QuizBattleDeckRanking || !currentCollection) return [];
    const matchID = currentMatch && currentMatch.id;
    return QuizBattleDeckRanking.rankEligibleCards(currentCollection.cards, matchID);
  }

  function renderDeckAssist(choosing) {
    const assist = document.getElementById("deckAssist");
    const autoButton = document.getElementById("autoSelectDeckButton");
    const clearButton = document.getElementById("clearDeckSelectionButton");
    if (assist) assist.hidden = !choosing;
    if (!choosing) return;
    const eligibleCount = eligibleDeckCards().length;
    if (autoButton) {
      autoButton.disabled = eligibleCount < 5;
      autoButton.title = eligibleCount < 5
        ? "تحتاج خمس بطاقات متاحة على الأقل"
        : "يختار أقوى خمس بطاقات دون تثبيتها";
    }
    if (clearButton) clearButton.disabled = selectedCards.size === 0;
    setText("deckAssistStatus", eligibleCount < 5
      ? "لديك " + eligibleCount + " بطاقات صالحة فقط؛ تحتاج خمس بطاقات للتجهيز."
      : deckAssistMessage);
  }

  function autoSelectDeck() {
    if (!window.QuizBattleDeckRanking || !currentCollection || !currentMatch) {
      deckAssistMessage = "تعذر ترتيب البطاقات الآن. أعد تحميل الساحة وحاول مرة أخرى.";
      renderDeckAssist(true);
      return;
    }
    const strongest = QuizBattleDeckRanking.strongestCards(currentCollection.cards, currentMatch.id, 5);
    if (strongest.length < 5) {
      deckAssistMessage = "لا توجد خمس بطاقات متاحة لهذه المباراة.";
      renderDeckAssist(true);
      return;
    }
    selectedCards.clear();
    strongest.forEach(function (card) { selectedCards.add(String(card.id)); });
    deckAssistMessage = "اخترنا أقوى خمس بطاقات. راجعها وعدّلها إن أردت، ثم اضغط تثبيت البطاقات.";
    renderDecks(matchPlayer(currentAccount.userId));
    updateCommitButton();
  }

  function clearDeckSelection() {
    selectedCards.clear();
    deckAssistMessage = "تم مسح الاختيار. اختر بطاقاتك يدويًا أو استخدم التجهيز التلقائي.";
    renderDecks(matchPlayer(currentAccount.userId));
    updateCommitButton();
  }

  function deckSlot(label, hidden, card) {
    const slot = document.createElement("span");
    slot.className = "qb-deck-slot";
    if (card && window.QuizBattleCardVisuals) {
      slot.classList.add("has-art", "rarity--" + card.rarity);
      const art = QuizBattleCardVisuals.createArt(card.category);
      art.classList.add("qb-deck-slot__art");
      const copy = document.createElement("span");
      copy.textContent = label;
      slot.append(art, copy);
    } else {
      if (hidden) slot.classList.add("is-card-back");
      slot.textContent = label;
    }
    if (hidden) slot.setAttribute("aria-label", label);
    return slot;
  }

  function ensureCommitButton() {
    let controls = document.getElementById("deckCommitControls");
    if (!controls) {
      controls = document.createElement("div");
      controls.id = "deckCommitControls";
      controls.className = "qb-actions";
      const button = document.createElement("button");
      button.id = "commitDeckButton";
      button.type = "button";
      button.className = "qb-button qb-button--primary";
      button.addEventListener("click", commitDeck);
      const help = document.createElement("span");
      help.id = "deckSelectionCount";
      help.className = "qb-help";
      controls.append(button, help);
      const deck = document.getElementById("playerDeck");
      deck.parentElement.appendChild(controls);
    }
    updateCommitButton();
  }

  function updateCommitButton() {
    const button = document.getElementById("commitDeckButton");
    if (button) {
      button.textContent = currentMatch && matchPlayer(currentAccount.userId) && matchPlayer(currentAccount.userId).deckReady ? "تحديث البطاقات الجاهزة" : "تثبيت البطاقات وإعلان الجاهزية";
      button.disabled = selectedCards.size !== 5;
    }
    setText("deckSelectionCount", selectedCards.size === 5
      ? "اكتملت المجموعة — يمكنك تثبيتها أو إزالة بطاقة للتبديل"
      : "اخترت " + selectedCards.size + " من 5");
  }

  function removeCommitButton() {
    const controls = document.getElementById("deckCommitControls");
    if (controls) controls.remove();
  }

  function startBlockerMessage(blockers) {
    const labels = {
      missing_players: "بانتظار اكتمال المقاعد المطلوبة.",
      not_enough_players: "تحتاج الساحة لاعبين على الأقل.",
      teams_not_full: "بانتظار اكتمال مقاعد الفريقين.",
      unbalanced_teams: "يجب أن يكون عدد اللاعبين متساويًا في الفريقين.",
      invalid_roster: "قائمة اللاعبين لا تطابق قواعد هذا النوع من الساحات.",
      invalid_state: "الساحة ليست في مرحلة تسمح بالبدء.",
      decks_not_ready: "بانتظار تثبيت بطاقات جميع اللاعبين.",
      players_not_ready: "بانتظار جاهزية جميع اللاعبين.",
      already_started: "بدأت المنافسة بالفعل.",
      not_owner: "مالك الساحة وحده يستطيع بدء المنافسة.",
    };
    const values = Array.isArray(blockers) ? blockers : [];
    const messages = values.map(function (value) {
      const key = String(value);
      if (key.startsWith("deck_not_ready:")) return "بانتظار تثبيت بطاقات " + playerName(key.split(":")[1]) + ".";
      return labels[key] || "";
    }).filter(Boolean);
    return messages[0] || "بانتظار اكتمال شروط البدء من الخادم.";
  }

  function renderPrepareButton() {
    const prepare = document.getElementById("preparegameButton");
    if (!prepare || !currentGame || !currentAccount) return;
    const owner = isOwner();
    const config = modeConfig();
    const count = joinedUsers().length;
    const eligible = gameMode() === "open"
      ? count >= config.minPlayers && count <= config.maxPlayers
      : count === config.maxPlayers;
    const preparing = !currentMatch && (!currentGame.state || currentGame.state === "lobby");
    prepare.hidden = !(owner && preparing);
    prepare.disabled = !eligible;
    prepare.textContent = isBotMode() ? "تجهيز مواجهة البوت" : "تجهيز الساحة";
    prepare.title = eligible
      ? (isBotMode() ? "افتح اختيار بطاقاتك وجهّز مجموعة البوت" : "ثبّت المشاركين وافتح اختيار البطاقات")
      : (gameMode() === "open" ? "تحتاج لاعبين على الأقل" : "يجب اكتمال " + config.maxPlayers + " مقاعد");
    if (!preparing) return;
    setText("startRequirement", owner
      ? (eligible ? "المقاعد مناسبة؛ جهّز الساحة ليفتح اختيار البطاقات." : prepare.title)
      : "بانتظار مالك الساحة لتجهيز المشاركين.");
  }

  function renderStartButton() {
    const start = document.getElementById("startgameButton");
    if (!start || !currentGame || !currentAccount) return;
    const owner = isOwner();
    const collecting = currentMatch && currentMatch.status === "collecting_decks";
    start.hidden = !(owner && collecting);
    if (!collecting) {
      if (currentMatch) setText("startRequirement", "");
      return;
    }
    const canStart = currentMatch.canStart === true;
    start.disabled = !canStart;
    start.textContent = isBotMode() ? "ابدأ مواجهة البوت" : "بدء المباراة";
    start.title = canStart ? "ابدأ المنافسة الآن" : startBlockerMessage(currentMatch.startBlockers);
    const readyCount = matchPlayers().filter(function (player) { return player.deckReady; }).length;
    setText("startRequirement", owner
      ? (canStart ? (isBotMode() ? "أنت وبوت المعرفة جاهزان — ابدأ المواجهة الآن." : "الجميع جاهز — يمكنك بدء المنافسة الآن.") : start.title)
      : readyCount + " من " + matchPlayers().length + " لاعبين جاهزون · البدء بيد المالك.");
  }

  function renderWaitingQuestion(message) {
    lastRenderedTurnKey = "waiting:" + message;
    clearInterval(clockTimer);
    clockTimer = null;
    setQuestionVisual("general-knowledge", null, null);
    setText("questionCategory", "تجهيز المباراة");
    setText("questionText", message);
    const options = document.getElementById("answerOptions");
    if (options) {
      options.disabled = true;
      options.replaceChildren();
    }
    setText("questionResult", "");
    renderClock(null);
  }

  function renderQuestion(turn) {
    const turnKey = [
      turn.id,
      turn.status,
      turn.yourOption,
      turn.correctOption,
      turn.deadline,
      turn.prompt,
      turn.canAnswer,
      (turn.eligibleUserIds || []).join(","),
      (turn.options || []).join("\u001f"),
    ].join("\u001e");
    const options = document.getElementById("answerOptions");
    if (turnKey === lastRenderedTurnKey) {
      if (options && sameID(answerSubmittingTurnID, turn.id)) options.disabled = true;
      return;
    }
    lastRenderedTurnKey = turnKey;
    setQuestionVisual(turn.category, turn.rarity, turn.power);
    setText("questionCategory", categoryLabel(turn.category) + " · " + difficultyLabel(turn.difficulty));
    setText("questionText", turn.prompt);
    if (!options) return;
    options.replaceChildren();
    const answered = turn.yourOption !== undefined && turn.yourOption !== null;
    const canAnswer = turn.canAnswer === true;
    (turn.options || []).forEach(function (text, index) {
      const answer = document.createElement("button");
      answer.type = "button";
      answer.className = "qb-answer";
      const selected = answered && Number(turn.yourOption) === index;
      answer.disabled = !canAnswer || answered || turn.status !== "active" || sameID(answerSubmittingTurnID, turn.id);
      answer.classList.toggle("is-selected", selected);
      answer.setAttribute("aria-pressed", selected ? "true" : "false");
      if (turn.status === "resolved" && Number(turn.correctOption) === index) answer.classList.add("is-correct");
      if (turn.status === "resolved" && selected && Number(turn.correctOption) !== index) answer.classList.add("is-wrong");
      answer.addEventListener("click", function () { submitAnswer(turn.id, index); });
      const span = document.createElement("span");
      span.textContent = text;
      answer.appendChild(span);
      options.appendChild(answer);
    });
    options.disabled = !canAnswer || answered || turn.status !== "active" || sameID(answerSubmittingTurnID, turn.id);
    if (turn.status === "resolved") {
      const ownAnswer = (turn.answers || []).find(function (answer) { return sameID(answer.userId, currentAccount.userId); });
      setText("questionResult", (ownAnswer
        ? (ownAnswer.correct ? "إجابة صحيحة" : "إجابة غير صحيحة") + " · +" + ownAnswer.points + " نقطة. "
        : (isTieBreakTurn(turn) && !canAnswer ? "شاهدت سؤال الحسم دون مشاركة. " : "انتهت المهلة دون إجابة. ")) + (turn.explanation || ""));
    } else {
      setText("questionResult", answered
        ? (isBotMode() ? "تم تسجيل إجابتك. بانتظار قرار بوت المعرفة أو انتهاء المهلة." : "تم تسجيل إجابتك. بانتظار بقية اللاعبين أو انتهاء المهلة.")
        : (!canAnswer
          ? "أنت تشاهد جولة الحسم؛ الإجابة متاحة للمتنافسين المتعادلين فقط."
          : "اختر إجابة واحدة قبل انتهاء الوقت."));
    }
    renderClock(turn);
    revealQuestion(turn);
  }

  function revealQuestion(turn) {
    if (!turn || turn.status !== "active" || sameID(lastRevealedTurnID, turn.id)) return;
    lastRevealedTurnID = String(turn.id);
    if (!window.matchMedia("(max-width: 63.99rem)").matches) return;
    const card = document.getElementById("questionCard");
    if (!card) return;
    window.requestAnimationFrame(function () {
      if (!sameID(lastRevealedTurnID, turn.id)) return;
      const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      card.scrollIntoView({ behavior: reducedMotion ? "auto" : "smooth", block: "start" });
    });
  }

  function setQuestionVisual(category, rarity, power) {
    const art = document.getElementById("questionArt");
    if (art && window.QuizBattleCardVisuals) QuizBattleCardVisuals.applyArt(art, category, { eager: true });
    setText("questionRarity", rarity ? rarityLabel(rarity) : "بطاقة الاستعداد");
    setText("questionPower", power ? "المستوى " + power : "الخادم يختار البطاقة");
    const card = document.getElementById("questionCard");
    if (!card) return;
    ["common", "rare", "epic", "legendary"].forEach(function (value) { card.classList.remove("rarity--" + value); });
    if (rarity) card.classList.add("rarity--" + rarity);
  }

  function renderClock(turn) {
    clearInterval(clockTimer);
    clockTimer = null;
    const container = document.getElementById("questionTimer");
    if (!container) return;
    container.replaceChildren();
    const text = document.createElement("span");
    const progress = document.createElement("progress");
    progress.max = 20000;
    progress.value = 0;
    progress.setAttribute("aria-label", "الوقت المتبقي للإجابة");
    container.append(text, progress);
    if (!turn || turn.status !== "active") {
      text.textContent = turn && turn.status === "resolved" ? "أُغلق الدور" : "المؤقت غير نشط";
      return;
    }
    function tick() {
      const remaining = Math.max(0, new Date(turn.deadline).getTime() - Date.now());
      progress.value = remaining;
      text.textContent = "الوقت المتبقي: " + (remaining / 1000).toFixed(1) + " ث";
    }
    tick();
    clockTimer = setInterval(tick, 200);
  }

  function renderLog(items) {
    const log = document.getElementById("matchLog");
    if (!log) return;
    const key = items.join("\u001f");
    if (key === lastLogKey) return;
    lastLogKey = key;
    log.replaceChildren();
    items.forEach(function (item) {
      const row = document.createElement("li");
      row.textContent = item;
      log.appendChild(row);
    });
  }

  async function loadCollection() {
    try {
      currentCollection = await QuizBattle.request("/api/v1/collection");
    } catch (error) {
      QuizBattle.showError("errorSumm", error);
    }
  }

  async function loadAll() {
    if (!battleID || stopped) return;
    if (loadInFlight) {
      loadPending = true;
      return;
    }
    loadInFlight = true;
    try {
      const results = await Promise.all([
        QuizBattle.request("/api/v1/game/" + battleID),
        QuizBattle.getSession(),
        QuizBattle.request("/api/v1/collection"),
      ]);
      currentGame = results[0];
      currentAccount = results[1];
      currentCollection = results[2];
      renderGame();
      await loadMatch(false);
    } catch (error) {
      if ([401, 403, 404, 410].includes(error.status)) {
        stopRealtime();
        window.location.replace(error.status === 401 ? "/auth/signin" : "/game/play");
        return;
      }
      QuizBattle.showError("errorSumm", error);
    } finally {
      loadInFlight = false;
      if (loadPending) {
        loadPending = false;
        loadAll();
      }
    }
  }

  async function loadMatch(showErrors) {
    if (!battleID || stopped) return;
    try {
      acceptMatchSnapshot(await QuizBattle.request("/api/v1/game/" + battleID + "/match"));
    } catch (error) {
      if (error.status === 404) acceptMatchSnapshot(null);
      else if (showErrors !== false) QuizBattle.showError("errorSumm", error);
    }
    renderMatch();
  }

  async function commitDeck() {
    const button = document.getElementById("commitDeckButton");
    if (button) button.disabled = true;
    try {
      acceptMatchSnapshot(await QuizBattle.request("/api/v1/game/" + battleID + "/deck", {
        method: "PUT",
        body: { cardIds: Array.from(selectedCards), commandId: commandID("deck") },
      }));
      await loadCollection();
      renderMatch();
    } catch (error) {
      QuizBattle.showError("errorSumm", error);
    } finally {
      updateCommitButton();
    }
  }

  async function prepareMatch() {
    const button = document.getElementById("preparegameButton");
    if (button) {
      button.disabled = true;
      button.setAttribute("aria-busy", "true");
    }
    try {
      const snapshot = await QuizBattle.request("/api/v1/game/" + battleID + "/prepare", {
        method: "POST", body: { commandId: commandID("prepare") },
      });
      if (snapshot && Array.isArray(snapshot.players)) acceptMatchSnapshot(snapshot);
      await loadAll();
    } catch (error) {
      QuizBattle.showError("errorSumm", error);
      await loadAll();
    } finally {
      if (button) button.removeAttribute("aria-busy");
      renderMatch();
    }
  }

  async function startMatch() {
    if (!currentMatch || currentMatch.canStart !== true || !isOwner()) return;
    const button = document.getElementById("startgameButton");
    if (button) button.disabled = true;
    try {
      acceptMatchSnapshot(await QuizBattle.request("/api/v1/game/" + battleID + "/start", {
        method: "POST", body: { commandId: commandID("start") },
      }));
      renderMatch();
    } catch (error) {
      QuizBattle.showError("errorSumm", error);
      await loadMatch(false);
    } finally {
      renderMatch();
    }
  }

  async function submitAnswer(turnID, option) {
    const activeTurn = currentMatch && currentMatch.currentTurn;
    if (!activeTurn || !sameID(activeTurn.id, turnID) || activeTurn.canAnswer !== true) return;
    const requestTurnID = String(turnID);
    if (sameID(answerSubmittingTurnID, requestTurnID)) return;
    answerSubmittingTurnID = requestTurnID;
    const fieldset = document.getElementById("answerOptions");
    if (fieldset) fieldset.disabled = true;
    setText("questionResult", "جاري تثبيت إجابتك لدى الخادم…");
    try {
      acceptMatchSnapshot(await QuizBattle.request("/api/v1/game/" + battleID + "/answer", {
        method: "POST", body: { turnId: turnID, option: option, commandId: commandID("answer") },
      }));
    } catch (error) {
      const refreshedTurn = currentMatch && currentMatch.currentTurn;
      if (!refreshedTurn || sameID(refreshedTurn.id, requestTurnID)) {
        QuizBattle.showError("errorSumm", error);
        await loadMatch(false);
      }
    } finally {
      if (sameID(answerSubmittingTurnID, requestTurnID)) answerSubmittingTurnID = null;
      lastRenderedTurnKey = null;
      renderMatch();
    }
  }

  async function exitBattle() {
    const inProgress = currentMatch && ["collecting_decks", "active", "tie_break"].includes(currentMatch.status);
    const owner = currentGame && currentAccount && sameID(currentGame.owner.id, currentAccount.userId);
    if (inProgress) {
      const botMode = isBotMode();
      const ownerOnlyCancellation = !isDuelMode();
      if (ownerOnlyCancellation && !owner) {
        QuizBattle.showError("errorSumm", new Error("لا يستطيع إلغاء الساحة الجماعية إلا مالكها."));
        return;
      }
      const confirmation = ownerOnlyCancellation
        ? "سيؤدي إلغاء المنافسة إلى إنهائها لجميع اللاعبين وتحرير بطاقاتهم دون مكافآت. هل تريد المتابعة؟"
        : (botMode
          ? "ستنتهي مواجهة البوت وتُحرر بطاقاتك دون مكافأة. هل تريد المتابعة؟"
          : "سيُسجل انسحابك وتُطبق قواعد الساحة على النقاط والبطاقات. هل تريد المتابعة؟");
      if (!window.confirm(confirmation)) return;
      try {
        acceptMatchSnapshot(await QuizBattle.request("/api/v1/game/" + battleID + "/forfeit", {
          method: "POST", body: { commandId: commandID("forfeit") },
        }));
      } catch (error) {
        QuizBattle.showError("errorSumm", error);
        return;
      }
    } else if (owner && !window.confirm("مغادرتك كمالك ستغلق الساحة. هل تريد المتابعة؟")) {
      return;
    }
    try {
      await QuizBattle.request("/api/v1/game/" + battleID + "/exit", { method: "POST" });
      stopRealtime();
      window.location.replace("/game/play");
    } catch (error) {
      QuizBattle.showError("errorSumm", error);
    }
  }

  function toggleExpanded() {
    const joined = document.getElementById("joineduserssection");
    const battle = document.getElementById("battleviewsection");
    const button = document.getElementById("expandBattleButton");
    if (!joined || !battle || !button) return;
    joined.hidden = !joined.hidden;
    battle.classList.toggle("col-md-8", !joined.hidden);
    battle.classList.toggle("col-md-11", joined.hidden);
    button.setAttribute("aria-expanded", joined.hidden ? "false" : "true");
    button.textContent = joined.hidden ? "إظهار اللاعبين" : "توسيع اللوح";
  }

  function connectBattle() {
    if (stopped || !battleID || battleSocket) return;
    battleSocket = new WebSocket(QuizBattle.websocketURL("/ws/game/" + battleID));
    battleSocket.addEventListener("open", function () {
      reconnectAttempts = 0;
      setText("matchConnectionStatus", "متصل");
      QuizBattle.showError("errorSumm", null);
      if (voiceJoined) {
        announcedVoiceReadyFor = null;
        announceVoiceReady(true);
      }
      loadAll();
    });
    battleSocket.addEventListener("message", function (event) {
      try {
        const update = JSON.parse(event.data);
        if (VOICE_SIGNAL_TYPES.has(update.type)) {
          processVoiceSignal(update);
          return;
        }
        if (!sameID(update.gameId, battleID)) return;
        if (update.type === "closed") {
          stopRealtime();
          window.location.replace("/game/play");
          return;
        }
        if (["joined", "left", "prepared"].includes(update.type)) loadAll();
        else loadMatch(false);
      } catch (_) {}
    });
    battleSocket.addEventListener("close", async function (event) {
      battleSocket = null;
      setText("matchConnectionStatus", "غير متصل");
      if (stopped) return;
      if (voiceJoined || voiceJoining) {
        leaveVoice({ notify: false });
        setVoiceStatus("انقطع اتصال الساحة، لذلك أوقفنا الميكروفون. يمكنك الانضمام للصوت مجددًا بعد عودة الاتصال.", "waiting");
      }
      try {
        await QuizBattle.getSession(true);
      } catch (error) {
        if (error.status === 401) {
          stopRealtime();
          window.location.replace("/auth/signin");
          return;
        }
      }
      try {
        await QuizBattle.request("/api/v1/game/" + battleID);
      } catch (error) {
        if ([401, 403, 404, 410].includes(error.status)) {
          stopped = true;
          window.location.replace(error.status === 401 ? "/auth/signin" : "/game/play");
          return;
        }
      }
      if (event.code === 1008) {
        stopped = true;
        renderVoiceControls();
        QuizBattle.showError("errorSumm", new Error("اتصال الساحة مستخدم في تبويب آخر أو لم يعد مسموحًا. أغلق التبويب الآخر ثم أعد تحميل هذه الصفحة."));
        return;
      }
      const delay = Math.min(30000, 1000 * Math.pow(2, reconnectAttempts++)) + Math.floor(Math.random() * 500);
      reconnectTimer = setTimeout(connectBattle, delay);
    });
  }

  function stopRealtime() {
    stopped = true;
    if (voiceJoined || voiceJoining) leaveVoice({ notify: true });
    clearTimeout(reconnectTimer);
    clearInterval(pollTimer);
    clearInterval(clockTimer);
    if (battleSocket) battleSocket.close();
  }

  const testAPI = Object.freeze({ normalizeMode, normalizeReward, rewardReasonLabel });
  if (typeof module !== "undefined" && module.exports) module.exports = testAPI;
  if (typeof window === "undefined" || typeof document === "undefined") return;

  window.QuizBattleBattleUI = testAPI;
  document.addEventListener("DOMContentLoaded", function () {
    battleID = parseBattleID();
    if (!battleID) {
      QuizBattle.showError("errorSumm", new Error("رقم الساحة غير صالح."));
      return;
    }
    document.getElementById("exitBattleButton").addEventListener("click", exitBattle);
    document.getElementById("expandBattleButton").addEventListener("click", toggleExpanded);
    document.getElementById("preparegameButton").addEventListener("click", prepareMatch);
    document.getElementById("startgameButton").addEventListener("click", startMatch);
    document.getElementById("autoSelectDeckButton").addEventListener("click", autoSelectDeck);
    document.getElementById("clearDeckSelectionButton").addEventListener("click", clearDeckSelection);
    document.getElementById("joinVoiceButton").addEventListener("click", joinVoice);
    document.getElementById("muteVoiceButton").addEventListener("click", toggleVoiceMute);
    document.getElementById("leaveVoiceButton").addEventListener("click", function () { leaveVoice({ notify: true }); });
    document.getElementById("resumeVoiceButton").addEventListener("click", resumeRemoteVoice);
    renderVoiceControls();
    connectBattle();
    loadAll();
    pollTimer = setInterval(function () {
      if (currentMatch && (["active", "tie_break"].includes(currentMatch.status) ||
        (["completed", "forfeited"].includes(currentMatch.status) && normalizeReward(currentMatch).status === "pending"))) loadMatch(false);
    }, 1000);
  });

  window.addEventListener("quizbattle:logout", stopRealtime);
  window.addEventListener("quizbattle:session-invalid", function () {
    stopRealtime();
    window.location.replace("/auth/signin");
  });
  window.addEventListener("pagehide", function () {
    if (voiceJoined || voiceJoining) leaveVoice({ notify: true });
  });
})();
