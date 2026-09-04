(function () {
  var KEY = "theme";
  var MODES = ["system", "light", "dark"];

  function readMode() {
    try {
      var v = localStorage.getItem(KEY);
      if (v === "light" || v === "dark" || v === "system") return v;
    } catch (_) {}
    return "system";
  }

  function writeMode(mode) {
    try {
      localStorage.setItem(KEY, mode);
    } catch (_) {}
  }

  function prefersDark() {
    return window.matchMedia("(prefers-color-scheme: dark)").matches;
  }

  function isDark(mode) {
    if (mode === "dark") return true;
    if (mode === "light") return false;
    return prefersDark();
  }

  function apply(mode) {
    var dark = isDark(mode);
    document.documentElement.classList.toggle("dark", dark);
    document.documentElement.setAttribute("data-theme-mode", mode);
    var meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.setAttribute("content", dark ? "#09090b" : "#111111");
    document.querySelectorAll("[data-theme-toggle]").forEach(function (btn) {
      btn.setAttribute("data-mode", mode);
      var label = btn.getAttribute("data-label-" + mode) || mode;
      btn.setAttribute("aria-label", label);
      btn.setAttribute("title", label);
    });
  }

  function cycle() {
    var cur = readMode();
    var next = MODES[(MODES.indexOf(cur) + 1) % MODES.length];
    writeMode(next);
    apply(next);
  }

  function bind() {
    document.querySelectorAll("[data-theme-toggle]").forEach(function (btn) {
      if (btn.dataset.bound === "1") return;
      btn.dataset.bound = "1";
      btn.addEventListener("click", function (e) {
        e.preventDefault();
        cycle();
      });
    });
  }

  apply(readMode());
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", bind);
  } else {
    bind();
  }

  try {
    window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", function () {
      if (readMode() === "system") apply("system");
    });
  } catch (_) {}
})();
