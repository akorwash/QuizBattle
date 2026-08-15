(function () {
  "use strict";

  const MAX_AVATAR_BYTES = 2 * 1024 * 1024;
  const ALLOWED_AVATAR_TYPES = new Set(["image/jpeg", "image/png"]);

  document.addEventListener("DOMContentLoaded", function () {
    const accountForm = document.getElementById("updateaccform");
    const avatarForm = document.getElementById("avatarForm");
    if (!accountForm && !avatarForm) return;

    const avatarEditor = document.getElementById("avatarEditor");
    const avatarInput = document.getElementById("avatarInput");
    const avatarChooseButton = document.getElementById("avatarChooseButton");
    const avatarUploadButton = document.getElementById("avatarUploadButton");
    const avatarDeleteButton = document.getElementById("avatarDeleteButton");
    const avatarStatus = document.getElementById("avatarStatus");
    const avatarPreviewFrame = document.getElementById("avatarPreviewFrame");
    const accountAvatarImage = document.getElementById("accountAvatarImage");
    const accountAvatarFallback = document.getElementById("accountAvatarFallback");
    const profileAvatarImage = document.getElementById("profileAvatarImage");
    const profileAvatarFallback = document.getElementById("profileAvatarFallback");

    let currentAccount = null;
    let currentAvatarURL = "";
    let selectedAvatarFile = null;
    let selectedAvatarReady = false;
    let selectedPreviewURL = "";
    let hasSavedAvatar = false;
    let avatarBusy = false;

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

    function setAvatarStatus(message, state) {
      if (!avatarStatus) return;
      avatarStatus.classList.toggle("is-success", state === "success");
      avatarStatus.classList.toggle("is-error", state === "error");
      avatarStatus.textContent = message || "";
    }

    function updateAvatarControls() {
      if (avatarInput) avatarInput.disabled = avatarBusy;
      if (avatarChooseButton) {
        avatarChooseButton.classList.toggle("is-disabled", avatarBusy);
        avatarChooseButton.setAttribute("aria-disabled", avatarBusy ? "true" : "false");
      }
      if (avatarUploadButton) avatarUploadButton.disabled = avatarBusy || !selectedAvatarFile || !selectedAvatarReady;
      if (avatarDeleteButton) avatarDeleteButton.disabled = avatarBusy || !hasSavedAvatar;
    }

    function setAvatarBusy(busy) {
      avatarBusy = busy;
      if (avatarEditor) avatarEditor.classList.toggle("is-busy", busy);
      if (avatarPreviewFrame) avatarPreviewFrame.setAttribute("aria-busy", busy ? "true" : "false");
      const loader = avatarPreviewFrame && avatarPreviewFrame.querySelector(".qb-avatar-editor__loader");
      if (loader) loader.hidden = !busy;
      updateAvatarControls();
    }

    function showFallback(image, fallback) {
      if (image) {
        image.hidden = true;
        image.removeAttribute("src");
      }
      if (fallback) fallback.hidden = false;
    }

    function showImage(image, fallback, source, alt, onResult) {
      if (!image || !source) {
        showFallback(image, fallback);
        if (onResult) onResult(false);
        return;
      }
      image.hidden = true;
      if (fallback) fallback.hidden = false;
      image.alt = alt || "";
      image.onload = function () {
        image.hidden = false;
        if (fallback) fallback.hidden = true;
        if (onResult) onResult(true);
      };
      image.onerror = function () {
        showFallback(image, fallback);
        if (onResult) onResult(false);
      };
      image.src = source;
    }

    function renderSavedAvatar(revision) {
      currentAvatarURL = avatarURL(currentAccount && currentAccount.userId, revision);
      const fullName = currentAccount && currentAccount.fullName ? currentAccount.fullName : "اللاعب";
      const alt = "صورة حساب " + fullName;

      showImage(profileAvatarImage, profileAvatarFallback, currentAvatarURL, alt);
      showImage(accountAvatarImage, accountAvatarFallback, currentAvatarURL, alt, function (loaded) {
        hasSavedAvatar = loaded;
        updateAvatarControls();
      });
    }

    function revokeSelectedPreview() {
      if (selectedPreviewURL.startsWith("blob:")) URL.revokeObjectURL(selectedPreviewURL);
      selectedPreviewURL = "";
    }

    function clearSelectedAvatar(restoreSavedAvatar) {
      selectedAvatarFile = null;
      selectedAvatarReady = false;
      if (avatarInput) avatarInput.value = "";
      revokeSelectedPreview();
      if (restoreSavedAvatar) renderSavedAvatar();
      updateAvatarControls();
    }

    function resetAllAvatarImages() {
      currentAvatarURL = "";
      hasSavedAvatar = false;
      showFallback(profileAvatarImage, profileAvatarFallback);
      showFallback(accountAvatarImage, accountAvatarFallback);
      updateAvatarControls();
    }

    function applyAccountIdentity(account, fillForm) {
      currentAccount = account;
      const name = account && account.fullName ? account.fullName : "لاعب QuizBattle";
      const initials = initialsFor(name);
      if (profileAvatarFallback) profileAvatarFallback.textContent = initials;
      if (accountAvatarFallback) accountAvatarFallback.textContent = initials;
      if (profileAvatarImage) profileAvatarImage.alt = "صورة حساب " + name;
      if (accountAvatarImage) accountAvatarImage.alt = "صورة حساب " + name;
      document.title = "QuizBattle — " + name;

      if (fillForm && accountForm) {
        accountForm.elements.inputname.value = account.fullName || "";
        if (account.yearOfBirth && account.monthOfBirth && account.dayOfBirth) {
          const pad = function (value) { return String(value).padStart(2, "0"); };
          accountForm.elements.inputdob.value = account.yearOfBirth + "-" + pad(account.monthOfBirth) + "-" + pad(account.dayOfBirth);
        }
      }
    }

    async function avatarRequest(method, formData) {
      const controller = new AbortController();
      const timeout = setTimeout(function () { controller.abort(); }, 15000);
      let response;
      try {
        response = await fetch("/api/v1/user/avatar", {
          method: method,
          credentials: "same-origin",
          headers: { Accept: "application/json" },
          body: formData || undefined,
          signal: controller.signal,
        });
      } catch (error) {
        if (error && error.name === "AbortError") throw new Error("انتهت مهلة رفع الصورة. تحقق من الاتصال ثم حاول مجددًا.");
        throw error;
      } finally {
        clearTimeout(timeout);
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
        if (response.status === 401) window.dispatchEvent(new Event("quizbattle:session-invalid"));
        let message = payload && payload.error ? payload.error : "تعذر تحديث صورة الحساب.";
        if (response.status === 413) message = "حجم الصورة أكبر من 2 ميجابايت. اختر صورة أصغر.";
        const error = new Error(message);
        error.status = response.status;
        throw error;
      }
      return payload;
    }

    function validateAvatar(file) {
      if (!file) return "اختر صورة أولًا.";
      if (!ALLOWED_AVATAR_TYPES.has(file.type)) return "صيغة الصورة غير مدعومة. اختر ملف JPG أو PNG فقط.";
      if (file.size === 0) return "ملف الصورة فارغ. اختر صورة أخرى.";
      if (file.size > MAX_AVATAR_BYTES) return "حجم الصورة أكبر من 2 ميجابايت. اختر صورة أصغر.";
      return "";
    }

    if (avatarInput) {
      avatarInput.addEventListener("change", function () {
        const file = avatarInput.files && avatarInput.files[0];
        const validationMessage = validateAvatar(file);
        if (validationMessage) {
          clearSelectedAvatar(true);
          setAvatarStatus(validationMessage, "error");
          return;
        }

        revokeSelectedPreview();
        selectedAvatarFile = file;
        selectedAvatarReady = false;
        setAvatarStatus("جاري تجهيز المعاينة…", "loading");
        const reader = new FileReader();
        reader.addEventListener("load", function () {
          if (selectedAvatarFile !== file) return;
          selectedPreviewURL = typeof reader.result === "string" ? reader.result : "";
          showImage(accountAvatarImage, accountAvatarFallback, selectedPreviewURL, "معاينة صورة الحساب الجديدة", function (loaded) {
            if (!loaded) {
              clearSelectedAvatar(true);
              setAvatarStatus("تعذر قراءة ملف الصورة. اختر صورة JPG أو PNG سليمة.", "error");
              return;
            }
            selectedAvatarReady = true;
            setAvatarStatus("المعاينة جاهزة. اضغط «حفظ الصورة» لتطبيقها.", "success");
            updateAvatarControls();
          });
        });
        reader.addEventListener("error", function () {
          if (selectedAvatarFile !== file) return;
          clearSelectedAvatar(true);
          setAvatarStatus("تعذر قراءة ملف الصورة. اختر صورة JPG أو PNG سليمة.", "error");
        });
        reader.readAsDataURL(file);
        updateAvatarControls();
      });
    }

    if (avatarChooseButton) {
      avatarChooseButton.addEventListener("click", function () {
        if (avatarBusy) return;
        if (avatarInput) avatarInput.click();
      });
    }

    if (avatarForm) {
      avatarForm.addEventListener("submit", async function (event) {
        event.preventDefault();
        const validationMessage = validateAvatar(selectedAvatarFile);
        if (validationMessage) {
          setAvatarStatus(validationMessage, "error");
          return;
        }
        if (!selectedAvatarReady) {
          setAvatarStatus("انتظر حتى تكتمل معاينة الصورة ثم حاول مجددًا.", "error");
          return;
        }

        const data = new FormData();
        data.append("avatar", selectedAvatarFile, selectedAvatarFile.name);
        setAvatarBusy(true);
        setAvatarStatus("جاري حفظ الصورة…", "loading");
        try {
          await avatarRequest("PUT", data);
          selectedAvatarFile = null;
          selectedAvatarReady = false;
          if (avatarInput) avatarInput.value = "";
          const oldPreviewURL = selectedPreviewURL;
          selectedPreviewURL = "";
          hasSavedAvatar = true;
          renderSavedAvatar(Date.now());
          if (oldPreviewURL.startsWith("blob:")) URL.revokeObjectURL(oldPreviewURL);
          setAvatarStatus("تم تحديث صورة حسابك بنجاح.", "success");
        } catch (error) {
          setAvatarStatus(error.message || "تعذر حفظ الصورة. حاول مجددًا.", "error");
        } finally {
          setAvatarBusy(false);
        }
      });
    }

    if (avatarDeleteButton) {
      avatarDeleteButton.addEventListener("click", async function () {
        if (!hasSavedAvatar || avatarBusy) return;
        if (!window.confirm("هل تريد حذف صورة حسابك والعودة إلى الأحرف الافتراضية؟")) return;

        setAvatarBusy(true);
        setAvatarStatus("جاري حذف الصورة…", "loading");
        try {
          await avatarRequest("DELETE");
          clearSelectedAvatar(false);
          resetAllAvatarImages();
          setAvatarStatus("تم حذف صورة الحساب.", "success");
        } catch (error) {
          setAvatarStatus(error.message || "تعذر حذف الصورة. حاول مجددًا.", "error");
        } finally {
          setAvatarBusy(false);
        }
      });
    }

    if (accountForm) {
      accountForm.addEventListener("submit", async function (event) {
        event.preventDefault();
        const date = new Date(accountForm.elements.inputdob.value + "T00:00:00Z");
        if (Number.isNaN(date.getTime())) {
          QuizBattle.showError("accountError", new Error("اختر تاريخ ميلاد صحيحًا."));
          return;
        }
        try {
          const account = await QuizBattle.request("/api/v1/user", {
            method: "POST",
            body: {
              fullName: accountForm.elements.inputname.value,
              yearOfBirth: date.getUTCFullYear(),
              monthOfBirth: date.getUTCMonth() + 1,
              dayOfBirth: date.getUTCDate(),
            },
          });
          QuizBattle.setSession(account);
          applyAccountIdentity(account, false);
          QuizBattle.showError("accountError", null);
          const status = document.getElementById("accountError");
          if (status) {
            status.classList.add("is-success");
            status.textContent = "تم حفظ بيانات الحساب بنجاح.";
          }
        } catch (error) {
          const status = document.getElementById("accountError");
          if (status) status.classList.remove("is-success");
          QuizBattle.showError("accountError", error);
        }
      });
    }

    QuizBattle.getSession().then(function (account) {
      applyAccountIdentity(account, true);
      renderSavedAvatar();
    }).catch(function (error) {
      QuizBattle.showError("accountError", error);
    });

    window.addEventListener("beforeunload", revokeSelectedPreview);
  });
})();
