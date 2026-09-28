(function () {
  try {
    var t = localStorage.getItem("theme");
    var d = t === "dark" || (t !== "light" && window.matchMedia("(prefers-color-scheme: dark)").matches);
    if (d) document.documentElement.classList.add("dark");
    document.documentElement.setAttribute("data-theme-mode", t || "system");
  } catch (e) {}

  // Restore language preference from settings FAB (localStorage) before paint.
  try {
    var stored = localStorage.getItem("llms_lang");
    if (!stored || !/^(ko|en|ja|zh)$/.test(stored)) return;
    var cur = (document.documentElement.getAttribute("lang") || "ko").toLowerCase();
    if (stored === cur) return;
    var segs = location.pathname.split("/").filter(Boolean);
    if (segs[0] === "ko" || segs[0] === "en" || segs[0] === "ja" || segs[0] === "zh") segs.shift();
    var rest = segs.join("/");
    var next = rest ? "/" + stored + "/" + rest : "/" + stored;
    if (next === location.pathname) return;
    location.replace(next + location.search + location.hash);
  } catch (e) {}
})();
