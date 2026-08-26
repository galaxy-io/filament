// Theme-aware connector icons.
// Mintlify hardcodes the -dark.svg CDN icon from page frontmatter into an
// <img>. Keep each icon in sync with the active theme (.dark class on <html>)
// by swapping -dark.svg <-> -light.svg. Runs on theme toggle and on
// client-side navigation. theme.css handles the instant first paint in
// Chromium/Safari; this covers everything else.
(function () {
  var CDN = "cdn.getgalaxy.io/sources/";

  function sync() {
    var dark = document.documentElement.classList.contains("dark");
    var want = dark ? "-dark.svg" : "-light.svg";
    var imgs = document.querySelectorAll('img[src*="' + CDN + '"]');
    for (var i = 0; i < imgs.length; i++) {
      var src = imgs[i].getAttribute("src");
      var have = src.slice(src.lastIndexOf("-"));
      if ((have === "-dark.svg" || have === "-light.svg") && have !== want) {
        imgs[i].setAttribute("src", src.slice(0, -have.length) + want);
      }
    }
  }

  new MutationObserver(sync).observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["class"],
    childList: true,
    subtree: true,
  });
  sync();
})();
