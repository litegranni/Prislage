/**
 * Prisläge bookmarklet — CSP-safe capture.
 *
 * Körs på butikssidan. Gör INGEN fetch till API:t (connect-src blockerar ofta).
 * Öppnar {APP_URL}/capture och skickar JSON-LD via postMessage.
 *
 * Rekommenderat bokmärke (byt APP_URL): se README eller hjälptexten i PWA:n.
 * Denna fil kan också laddas om butiken tillåter script-src till din app.
 */
(function () {
  var APP_URL = (
    window.__PRISLAGE_APP_URL__ ||
    localStorage.getItem("prislage_app_url") ||
    ""
  ).replace(/\/$/, "");

  if (!APP_URL) {
    // Om skriptet laddats från PWA:n, använd den origin.
    var scripts = document.getElementsByTagName("script");
    for (var i = scripts.length - 1; i >= 0; i--) {
      var src = scripts[i].src || "";
      if (src.indexOf("/capture.js") !== -1) {
        try {
          APP_URL = new URL(src).origin;
        } catch (e) {}
        break;
      }
    }
  }

  if (!APP_URL) {
    APP_URL = prompt("Prisläge APP_URL (t.ex. https://prislage.your-tailnet.ts.net)", "") || "";
    APP_URL = APP_URL.replace(/\/$/, "");
    if (APP_URL) localStorage.setItem("prislage_app_url", APP_URL);
  }
  if (!APP_URL) {
    alert("Ingen APP_URL — avbryter.");
    return;
  }

  var appOrigin;
  try {
    appOrigin = new URL(APP_URL).origin;
  } catch (e) {
    alert("Ogiltig APP_URL");
    return;
  }

  var chunks = Array.prototype.slice
    .call(document.querySelectorAll('script[type="application/ld+json"]'))
    .map(function (el) {
      return (el.textContent || "").trim();
    })
    .filter(Boolean);

  var jsonld = "";
  if (chunks.length === 1) {
    jsonld = chunks[0];
  } else if (chunks.length > 1) {
    jsonld = JSON.stringify(
      chunks.map(function (c) {
        try {
          return JSON.parse(c);
        } catch (e) {
          return c;
        }
      })
    );
  }

  var payload = {
    type: "prislage-capture",
    url: location.href,
    title: document.title || "",
    jsonld: jsonld,
  };

  var w = window.open(APP_URL + "/capture", "prislage_capture");
  if (!w) {
    alert("Kunde inte öppna capture-fönster (popup blocker?).");
    return;
  }

  var tries = 0;
  var timer = setInterval(function () {
    tries += 1;
    if (!w || w.closed) {
      clearInterval(timer);
      return;
    }
    try {
      w.postMessage(payload, appOrigin);
    } catch (e) {}
    if (tries >= 40) clearInterval(timer);
  }, 150);

  function onAck(ev) {
    if (ev.origin !== appOrigin) return;
    if (!ev.data || ev.data.type !== "prislage-capture-ack") return;
    clearInterval(timer);
    window.removeEventListener("message", onAck);
  }
  window.addEventListener("message", onAck);
})();
