/**
 * Prisläge bookmarklet helper.
 *
 * Installera: skapa ett bokmärke vars URL är resultatet av:
 *   javascript:(function(){var s=document.createElement('script');s.src='http://localhost:4188/capture.js';document.body.appendChild(s);})();
 *
 * Kräver att API:t körs och att token sparats via localStorage:
 *   localStorage.setItem('prislage_api_base','http://localhost:3847');
 *   localStorage.setItem('prislage_api_token','din-token');
 */
(function () {
  var apiBase = localStorage.getItem("prislage_api_base") || "http://localhost:3847";
  var token = localStorage.getItem("prislage_api_token") || "";
  if (!token) {
    token = prompt("Prisläge API-token") || "";
    if (token) localStorage.setItem("prislage_api_token", token);
  }
  if (!token) {
    alert("Ingen token — avbryter.");
    return;
  }

  var scripts = Array.prototype.slice.call(
    document.querySelectorAll('script[type="application/ld+json"]')
  );
  var chunks = scripts
    .map(function (el) {
      return (el.textContent || "").trim();
    })
    .filter(Boolean);

  var payload = {
    url: location.href,
    html: "",
    jsonld: "",
  };

  if (chunks.length === 1) {
    payload.jsonld = chunks[0];
  } else if (chunks.length > 1) {
    payload.jsonld = JSON.stringify(chunks.map(function (c) {
      try { return JSON.parse(c); } catch (e) { return c; }
    }));
  } else {
    payload.html = document.documentElement.outerHTML;
  }

  fetch(apiBase.replace(/\/$/, "") + "/api/capture", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + token,
    },
    body: JSON.stringify(payload),
  })
    .then(function (res) {
      return res.json().then(function (body) {
        if (!res.ok) throw new Error(body.error || res.statusText);
        return body;
      });
    })
    .then(function (item) {
      alert(
        "Sparat: " +
          (item.name || "produkt") +
          (item.current_price != null ? " @ " + item.current_price : "")
      );
    })
    .catch(function (err) {
      alert("Prisläge capture misslyckades: " + err.message);
    });
})();
