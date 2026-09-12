(function () {
  var LANG_KEY = "llms_lang";

  function panelEl() {
    return document.getElementById("settings-panel");
  }

  function toggleBtn() {
    return document.querySelector("[data-settings-toggle]");
  }

  function isOpen() {
    var panel = panelEl();
    return panel && !panel.hasAttribute("hidden") && !panel.classList.contains("hidden");
  }

  function openPanel() {
    var panel = panelEl();
    var btn = toggleBtn();
    if (!panel || !btn) return;
    panel.hidden = false;
    panel.classList.remove("hidden");
    btn.setAttribute("aria-expanded", "true");
  }

  function closePanel() {
    var panel = panelEl();
    var btn = toggleBtn();
    if (!panel || !btn) return;
    panel.hidden = true;
    panel.classList.add("hidden");
    btn.setAttribute("aria-expanded", "false");
  }

  function readLang() {
    try {
      var v = localStorage.getItem(LANG_KEY);
      if (v === "ko" || v === "en" || v === "ja" || v === "zh") return v;
    } catch (_) {}
    var cur = (document.documentElement.getAttribute("lang") || "").toLowerCase();
    if (cur === "ko" || cur === "en" || cur === "ja" || cur === "zh") return cur;
    return "ko";
  }

  function writeLang(lang) {
    if (!(lang === "ko" || lang === "en" || lang === "ja" || lang === "zh")) return;
    try {
      localStorage.setItem(LANG_KEY, lang);
    } catch (_) {}
  }

  function syncLangSeg() {
    var lang = readLang();
    document.querySelectorAll(".lang-seg a[hreflang]").forEach(function (a) {
      var active = (a.getAttribute("hreflang") || "").toLowerCase() === lang;
      a.classList.toggle("is-active", active);
      if (active) a.setAttribute("aria-current", "true");
      else a.removeAttribute("aria-current");
    });
  }

  function bindLangPersistence() {
    // Mirror the page language so a direct URL visit also sticks in settings.
    var pageLang = (document.documentElement.getAttribute("lang") || "").toLowerCase();
    if (pageLang === "ko" || pageLang === "en" || pageLang === "ja" || pageLang === "zh") {
      writeLang(pageLang);
    }
    document.querySelectorAll(".lang-seg a[hreflang]").forEach(function (a) {
      if (a.dataset.langBound === "1") return;
      a.dataset.langBound = "1";
      a.addEventListener("click", function () {
        writeLang((a.getAttribute("hreflang") || "").toLowerCase());
      });
    });
    syncLangSeg();
  }

  function bindSettings() {
    var root = document.querySelector("[data-settings-fab]");
    var btn = toggleBtn();
    if (!root || !btn || btn.dataset.bound === "1") return;
    btn.dataset.bound = "1";
    btn.addEventListener("click", function (e) {
      e.preventDefault();
      e.stopPropagation();
      if (isOpen()) closePanel();
      else openPanel();
    });
    document.addEventListener("click", function (e) {
      if (!isOpen()) return;
      if (root.contains(e.target)) return;
      closePanel();
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape" && isOpen()) closePanel();
    });
  }

  document.addEventListener("click", function (e) {
    document.querySelectorAll("details.lang-dd[open]").forEach(function (d) {
      if (!d.contains(e.target)) d.removeAttribute("open");
    });
  });

  function boot() {
    bindSettings();
    bindLangPersistence();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
})();
