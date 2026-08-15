(function () {
  "use strict";

  function bindForm(form, buildPayload) {
    if (!form) return;
    form.addEventListener("submit", async function (event) {
      event.preventDefault();
      const errorElement = document.getElementById("errorSumm");
      QuizBattle.showError(errorElement, null);
      const button = form.querySelector("button[type='submit']");
      if (button) button.disabled = true;
      try {
        const account = await QuizBattle.request(form.dataset.endpoint, {
          method: "POST",
          body: buildPayload(form),
        });
        QuizBattle.setSession(account);
        window.location.assign("/");
      } catch (error) {
        QuizBattle.showError(errorElement, error);
      } finally {
        if (button) button.disabled = false;
      }
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    QuizBattle.getSession()
      .then(function () { window.location.replace("/"); })
      .catch(function () {});

    bindForm(document.getElementById("loginform"), function (form) {
      return {
        identifier: form.elements.inputemail.value,
        password: form.elements.inputpassword.value,
      };
    });
    bindForm(document.getElementById("signupform"), function (form) {
      return {
        fullName: form.elements.inputname.value,
        email: form.elements.inputemail.value,
        mobileNumber: form.elements.inputmobile.value,
        username: form.elements.inputusername.value,
        password: form.elements.inputpassword.value,
      };
    });
  });
})();
