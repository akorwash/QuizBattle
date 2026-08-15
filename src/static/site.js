(function () {
  "use strict";

  function updateNavigation(account) {
    const signIn = document.getElementById("signInNav");
    const profile = document.getElementById("useracc");
    const logout = document.getElementById("logout");
	if (!account) {
	  if (signIn) signIn.hidden = false;
	  if (profile) profile.hidden = true;
	  if (logout) logout.hidden = true;
	  return;
	}
    if (signIn) signIn.hidden = true;
    if (profile) {
      profile.hidden = false;
      profile.href = "/user/profile";
      profile.textContent = "مرحبًا، " + account.fullName;
    }
    if (logout) logout.hidden = false;
  }

  document.addEventListener("DOMContentLoaded", function () {
    const logout = document.getElementById("logout");
    if (logout) {
      logout.addEventListener("click", async function (event) {
        event.preventDefault();
        try {
          await QuizBattle.request("/user/logout", { method: "POST" });
		  window.dispatchEvent(new Event("quizbattle:logout"));
          QuizBattle.clearSession();
		  window.location.replace("/");
		} catch (error) {
		  logout.textContent = "تعذّر الخروج — أعد المحاولة";
		  logout.title = error.message || "تعذّر تسجيل الخروج";
		  QuizBattle.showError("errorSumm", error);
        }
      });
    }
    QuizBattle.getSession().then(updateNavigation).catch(function () { updateNavigation(null); });
  });

	window.addEventListener("quizbattle:session-changed", function (event) {
	  if (event.detail) updateNavigation(event.detail);
	});

	window.addEventListener("quizbattle:session-invalid", function () {
	  updateNavigation(null);
	});

	window.addEventListener("pageshow", function (event) {
	  if (event.persisted) window.location.reload();
	});
})();
