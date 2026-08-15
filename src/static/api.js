(function () {
  "use strict";

  let sessionPromise = null;

  async function request(path, options) {
	const settings = Object.assign({ credentials: "same-origin" }, options || {});
    settings.headers = Object.assign({ Accept: "application/json" }, settings.headers || {});
    if (settings.body && typeof settings.body !== "string") {
      settings.headers["Content-Type"] = "application/json";
      settings.body = JSON.stringify(settings.body);
    }

	let timeout = null;
	let controller = null;
	if (!settings.signal) {
	  controller = new AbortController();
	  settings.signal = controller.signal;
	  timeout = setTimeout(function () { controller.abort(); }, 15000);
	}
	let response;
	try {
	  response = await fetch(path, settings);
	} catch (error) {
	  if (error && error.name === "AbortError") throw new Error("انتهت مهلة الطلب. تحقق من الاتصال ثم حاول مجددًا.");
	  throw error;
	} finally {
	  if (timeout) clearTimeout(timeout);
	}
	const contentType = response.headers.get("Content-Type") || "";
	let payload = null;
	if (contentType.includes("application/json")) {
	  try {
		payload = await response.json();
	  } catch (_) {}
	} else if (!response.ok) {
	  const message = (await response.text()).trim().slice(0, 300);
	  payload = message ? { error: message } : null;
	}
	if (!response.ok) {
	  if (response.status === 401) {
		sessionPromise = null;
		window.dispatchEvent(new Event("quizbattle:session-invalid"));
	  }
	  let message = payload && payload.error ? payload.error : "تعذر تنفيذ الطلب.";
	  const retryAfter = response.headers.get("Retry-After");
	  if (response.status === 429 && retryAfter) message += " حاول مجددًا بعد " + retryAfter + " ثانية.";
	  const error = new Error(message);
	  error.status = response.status;
	  error.retryAfter = retryAfter;
      throw error;
    }
    return payload;
  }

  function getSession(refresh) {
    if (refresh || !sessionPromise) {
      sessionPromise = request("/api/v1/session").catch(function (error) {
        sessionPromise = null;
        throw error;
      });
    }
    return sessionPromise;
  }

  function setSession(account) {
    sessionPromise = Promise.resolve(account);
	window.dispatchEvent(new CustomEvent("quizbattle:session-changed", { detail: account }));
  }

  function clearSession() {
    sessionPromise = null;
  }

  function websocketURL(path) {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    return protocol + "//" + window.location.host + path;
  }

  function showError(elementOrID, error) {
    const element = typeof elementOrID === "string" ? document.getElementById(elementOrID) : elementOrID;
    if (element) {
      element.classList.remove("is-success");
      element.textContent = error ? error.message || String(error) : "";
    }
  }

  window.QuizBattle = { request, getSession, setSession, clearSession, websocketURL, showError };
})();
