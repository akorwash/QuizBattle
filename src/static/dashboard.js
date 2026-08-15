(function () {
  "use strict";

  document.addEventListener("DOMContentLoaded", function () {
    const tabList = document.querySelector("[data-dashboard-tabs]");
    if (!tabList) return;
    const tabs = Array.from(tabList.querySelectorAll('a[href^="#"]'));
    const panels = tabs.map(function (tab) {
      return document.getElementById(tab.getAttribute("href").slice(1));
    }).filter(Boolean);
    if (!tabs.length || !panels.length) return;

    tabList.setAttribute("role", "tablist");
    tabs.forEach(function (tab) {
      const panelID = tab.getAttribute("href").slice(1);
      tab.id = "dashboard-tab-" + panelID;
      tab.setAttribute("role", "tab");
      tab.setAttribute("aria-controls", panelID);
    });
    panels.forEach(function (panel) {
      panel.setAttribute("role", "tabpanel");
      panel.setAttribute("tabindex", "0");
      panel.setAttribute("aria-labelledby", "dashboard-tab-" + panel.id);
    });

    function activate(hash, moveFocus) {
      const targetID = (hash || "#collection").slice(1);
      const target = panels.find(function (panel) { return panel.id === targetID; }) || panels[0];
      panels.forEach(function (panel) { panel.hidden = panel !== target; });
      tabs.forEach(function (tab) {
        const selected = tab.getAttribute("href") === "#" + target.id;
        tab.classList.toggle("is-active", selected);
        tab.setAttribute("aria-selected", selected ? "true" : "false");
        tab.tabIndex = selected ? 0 : -1;
      });
      if (moveFocus) target.focus({ preventScroll: true });
    }

    tabs.forEach(function (tab) {
      tab.addEventListener("click", function (event) {
        event.preventDefault();
        const hash = tab.getAttribute("href");
        window.history.pushState(null, "", hash);
        activate(hash, false);
      });
      tab.addEventListener("keydown", function (event) {
        if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
        event.preventDefault();
        const current = tabs.indexOf(tab);
        const direction = event.key === "ArrowLeft" ? 1 : -1;
        const next = tabs[(current + direction + tabs.length) % tabs.length];
        next.click();
        next.focus();
      });
    });
    window.addEventListener("hashchange", function () { activate(window.location.hash, false); });
    activate(window.location.hash, false);
  });
})();
