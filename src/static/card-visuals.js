(function () {
  "use strict";

  const atlasURL = "/static/art/knowledge-categories-atlas.jpg?v=20260815.1";
  const categories = Object.freeze({
    mathematics: { label: "رياضيات", className: "category--mathematics" },
    geography: { label: "جغرافيا", className: "category--geography" },
    science: { label: "علوم", className: "category--science" },
    cities: { label: "مدن", className: "category--cities" },
    religion: { label: "معرفة دينية", className: "category--religion" },
    technology: { label: "تقنية", className: "category--technology" },
    civics: { label: "سياسة ومدنيات", className: "category--civics" },
    "general-knowledge": { label: "ثقافة عامة", className: "category--general-knowledge" },
    history: { label: "تاريخ", className: "category--history" },
  });
  const rarityLabels = Object.freeze({ common: "شائعة", rare: "نادرة", epic: "ملحمية", legendary: "أسطورية" });
  const difficultyLabels = Object.freeze({ easy: "سهل", medium: "متوسط", hard: "صعب" });

  function categoryKey(value) {
    return Object.prototype.hasOwnProperty.call(categories, value) ? value : "general-knowledge";
  }

  function categoryLabel(value) {
    const key = categoryKey(value);
    return categories[key].label;
  }

  function rarityLabel(value) {
    return rarityLabels[value] || value || "غير محددة";
  }

  function difficultyLabel(value) {
    return difficultyLabels[value] || value || "غير محدد";
  }

  function applyArt(container, category, options) {
    if (!container) return null;
    const settings = options || {};
    const key = categoryKey(category);
    Object.keys(categories).forEach(function (candidate) {
      container.classList.remove(categories[candidate].className);
    });
    container.classList.add("qb-card-art", categories[key].className);
    container.replaceChildren();
    const image = document.createElement("img");
    image.src = atlasURL;
    image.alt = "";
    image.width = 1200;
    image.height = 1200;
    image.decoding = "async";
    image.loading = settings.eager ? "eager" : "lazy";
    if (settings.eager) image.fetchPriority = "high";
    container.appendChild(image);
    // The image is decorative, while callers may place readable status text
    // inside the art frame. A presentation role keeps that text accessible.
    container.removeAttribute("aria-hidden");
    container.setAttribute("role", "presentation");
    return container;
  }

  function createArt(category, options) {
    return applyArt(document.createElement("div"), category, options);
  }

  window.QuizBattleCardVisuals = Object.freeze({
    atlasURL: atlasURL,
    applyArt: applyArt,
    categoryKey: categoryKey,
    categoryLabel: categoryLabel,
    createArt: createArt,
    difficultyLabel: difficultyLabel,
    rarityLabel: rarityLabel,
  });
})();
