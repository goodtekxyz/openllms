(function () {
  try {
    var t = localStorage.getItem("theme");
    var d = t === "dark" || (t !== "light" && window.matchMedia("(prefers-color-scheme: dark)").matches);
    if (d) document.documentElement.classList.add("dark");
    document.documentElement.setAttribute("data-theme-mode", t || "system");
  } catch (e) {}
})();
