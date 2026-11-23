/**
 * Prisläge bookmarklet — CSP-safe capture (handshake + fragment fallback).
 *
 * Prefer the inline javascript: bookmark from the PWA help text (avoids script-src).
 * Keep logic in sync with web/src/bookmarklet.ts.
 */
(function () {
  var APP_URL = (
    window.__PRISLAGE_APP_URL__ ||
    localStorage.getItem("prislage_app_url") ||
    ""
  ).replace(/\/$/, "");

  if (!APP_URL) {
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
    APP_URL = prompt("Prisläge APP_URL (https://…)", "") || "";
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

  function typeNames(t) {
    return Array.isArray(t) ? t.map(String) : [String(t || "")];
  }
  function normType(t) {
    return String(t).replace(/^https?:\/\/schema\.org\//i, "");
  }
  function want(node) {
    return typeNames(node["@type"]).some(function (t) {
      t = normType(t);
      return /^(Product|ProductGroup|IndividualProduct|Offer|AggregateOffer)$/i.test(t);
    });
  }
  function slimOffer(v) {
    if (Array.isArray(v)) return v.map(slimOffer);
    if (!v || typeof v !== "object") return v;
    var o = {};
    var keep = {
      price: 1,
      lowPrice: 1,
      highPrice: 1,
      priceCurrency: 1,
      priceSpecification: 1,
      availability: 1,
      url: 1,
      name: 1,
      "@type": 1,
    };
    for (var k in v) {
      if (keep[k]) o[k] = k === "priceSpecification" ? slimOffer(v[k]) : v[k];
    }
    return o;
  }
  function slimProduct(n) {
    var o = {};
    var keep = {
      "@context": 1,
      "@type": 1,
      name: 1,
      brand: 1,
      sku: 1,
      gtin: 1,
      gtin8: 1,
      gtin12: 1,
      gtin13: 1,
      gtin14: 1,
      mpn: 1,
      productID: 1,
      offers: 1,
    };
    for (var k in n) {
      if (!keep[k]) continue;
      if (k === "offers") o[k] = slimOffer(n[k]);
      else if (k === "brand" && n[k] && typeof n[k] === "object")
        o[k] = { "@type": "Brand", name: n[k].name || n[k] };
      else o[k] = n[k];
    }
    return o;
  }
  function trimLD(chunks) {
    var out = [];
    function walk(v) {
      if (Array.isArray(v)) {
        v.forEach(walk);
        return;
      }
      if (!v || typeof v !== "object") return;
      if (want(v)) {
        var ts = typeNames(v["@type"]).map(normType).join(" ");
        if (/Offer/i.test(ts) && !/Product/i.test(ts)) {
          out.push({ "@type": "Product", name: v.name || "", offers: slimOffer(v) });
        } else {
          out.push(slimProduct(v));
        }
      }
      if (v["@graph"]) walk(v["@graph"]);
      ["item", "itemListElement", "mainEntity", "hasVariant", "offers"].forEach(function (k) {
        if (v[k] != null) walk(v[k]);
      });
    }
    chunks.forEach(function (c) {
      try {
        walk(typeof c === "string" ? JSON.parse(c) : c);
      } catch (e) {}
    });
    if (!out.length) return "";
    return out.length === 1 ? JSON.stringify(out[0]) : JSON.stringify(out);
  }
  function b64url(s) {
    var u = unescape(encodeURIComponent(s));
    var b = btoa(u);
    return b.replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  }

  var chunks = Array.prototype.slice
    .call(document.querySelectorAll('script[type="application/ld+json"]'))
    .map(function (el) {
      return (el.textContent || "").trim();
    })
    .filter(Boolean);

  var payload = {
    type: "prislage-capture",
    url: location.href,
    title: document.title || "",
    jsonld: trimLD(chunks),
  };

  var w = window.open(APP_URL + "/capture", "prislage_capture");
  if (!w) {
    alert("Kunde inte öppna capture-fönster (popup blocker?).");
    return;
  }

  var phase = 0; // 0 waiting, 1 sent postMessage, 2 fallback
  function send() {
    if (phase) return;
    phase = 1;
    try {
      w.postMessage(payload, appOrigin);
    } catch (e) {
      fallback();
    }
  }
  function fallback() {
    if (phase === 2) return;
    phase = 2;
    var u = APP_URL + "/capture#d=" + b64url(JSON.stringify(payload));
    try {
      if (w && !w.closed) w.location.href = u;
      else window.open(u, "prislage_capture");
    } catch (e) {
      window.open(u, "prislage_capture");
    }
  }

  function onMsg(ev) {
    if (ev.origin !== appOrigin || !ev.data) return;
    if (ev.data.type === "prislage-ready") send();
    if (ev.data.type === "prislage-capture-ack") {
      clearTimeout(to);
      window.removeEventListener("message", onMsg);
    }
  }
  window.addEventListener("message", onMsg);

  var to = setTimeout(function () {
    window.removeEventListener("message", onMsg);
    fallback();
  }, 5000);
})();
