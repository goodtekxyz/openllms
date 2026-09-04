(function () {
  document.addEventListener("click", function (e) {
    document.querySelectorAll("details.lang-dd[open]").forEach(function (d) {
      if (!d.contains(e.target)) d.removeAttribute("open");
    });
  });
})();
