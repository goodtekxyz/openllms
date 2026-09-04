/**
 * One-click copy for install .shell / article <pre> blocks.
 */
(function () {
  var LABELS = {
    ko: { copy: "복사", copied: "복사됨" },
    en: { copy: "Copy", copied: "Copied" },
    ja: { copy: "コピー", copied: "コピー済み" },
    zh: { copy: "复制", copied: "已复制" },
  };

  var ICON =
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>';
  var CHECK =
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 6L9 17l-5-5"/></svg>';

  function labels() {
    var l = (document.documentElement.lang || "en").slice(0, 2).toLowerCase();
    return LABELS[l] || LABELS.en;
  }

  function textOf(el) {
    var code = el.querySelector && el.querySelector("code");
    return ((code || el).textContent || "").trim();
  }

  function copyText(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      return navigator.clipboard.writeText(text);
    }
    var ta = document.createElement("textarea");
    ta.value = text;
    ta.style.cssText = "position:fixed;left:-9999px";
    document.body.appendChild(ta);
    ta.select();
    try {
      document.execCommand("copy");
    } finally {
      document.body.removeChild(ta);
    }
    return Promise.resolve();
  }

  function enhance(el) {
    if (el.closest(".copy-block")) return;
    var wrap = document.createElement("div");
    wrap.className = "copy-block";
    el.parentNode.insertBefore(wrap, el);
    wrap.appendChild(el);

    var L = labels();
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "copy-btn";
    btn.setAttribute("aria-label", L.copy);
    btn.title = L.copy;
    btn.innerHTML = ICON;
    wrap.appendChild(btn);

    var timer;
    btn.onclick = function () {
      var t = textOf(el);
      if (!t) return;
      copyText(t).then(function () {
        btn.classList.add("is-copied");
        btn.setAttribute("aria-label", L.copied);
        btn.title = L.copied;
        btn.innerHTML = CHECK;
        clearTimeout(timer);
        timer = setTimeout(function () {
          btn.classList.remove("is-copied");
          btn.setAttribute("aria-label", L.copy);
          btn.title = L.copy;
          btn.innerHTML = ICON;
        }, 1200);
      });
    };
  }

  function run() {
    document.querySelectorAll(".shell, article pre").forEach(enhance);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", run);
  } else {
    run();
  }
})();
