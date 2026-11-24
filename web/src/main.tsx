import { render } from "preact";
import { useCallback, useEffect, useMemo, useState } from "preact/hooks";
import {
  api,
  money,
  safeHref,
  type AnalyzeResponse,
  type ItemSummary,
  type PricePoint,
} from "./api";
import { bookmarkletFor } from "./bookmarklet";
import { CapturePage } from "./CapturePage";
import "./styles.css";

function Sparkline({ values }: { values?: number[] }) {
  const pts = values && values.length > 1 ? values : null;
  if (!pts) {
    return <svg class="spark" viewBox="0 0 88 40" aria-hidden="true"><path d="M4 28h80" stroke="rgba(255,255,255,0.2)" stroke-width="2"/></svg>;
  }
  const min = Math.min(...pts);
  const max = Math.max(...pts);
  const span = max - min || 1;
  const d = pts
    .map((v, i) => {
      const x = (i / (pts.length - 1)) * 84 + 2;
      const y = 34 - ((v - min) / span) * 28;
      return `${i === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)}`;
    })
    .join(" ");
  return (
    <svg class="spark" viewBox="0 0 88 40" aria-hidden="true">
      <path d={d} fill="none" stroke="#c8f060" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
  );
}

function PriceChart({ points }: { points: PricePoint[] }) {
  const series = useMemo(() => points.map((p) => Number(p.price)), [points]);
  if (series.length < 2) {
    return <p class="meta">Behöver minst två pris-punkter för graf.</p>;
  }
  const min = Math.min(...series);
  const max = Math.max(...series);
  const span = max - min || 1;
  const w = 640;
  const h = 200;
  const pad = 16;
  const coords = series.map((v, i) => {
    const x = pad + (i / (series.length - 1)) * (w - pad * 2);
    const y = h - pad - ((v - min) / span) * (h - pad * 2);
    return [x, y] as const;
  });
  const d = coords.map(([x, y], i) => `${i === 0 ? "M" : "L"}${x} ${y}`).join(" ");
  const area = `${d} L${coords[coords.length - 1][0]} ${h - pad} L${coords[0][0]} ${h - pad} Z`;
  return (
    <div class="chart-wrap">
      <svg viewBox={`0 0 ${w} ${h}`} role="img" aria-label="Pris över tid">
        <path d={area} fill="rgba(200,240,96,0.12)" />
        <path d={d} fill="none" stroke="#c8f060" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" />
        {coords.map(([x, y], i) => (
          <circle key={points[i].id} cx={x} cy={y} r="3.5" fill="#f2f6f1" />
        ))}
      </svg>
      <div class="meta">
        Lägsta {money(min)} · Högsta {money(max)} · {points.length} mätpunkter
      </div>
    </div>
  );
}

function urlBase64ToUint8Array(base64String: string) {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  return Uint8Array.from([...raw].map((c) => c.charCodeAt(0)));
}

function confidenceLabel(c: string): string {
  switch (c) {
    case "high":
      return "Hög säkerhet";
    case "medium":
      return "Medel säkerhet";
    default:
      return "Låg säkerhet";
  }
}

function suggestedTarget(analysis: AnalyzeResponse): number {
  const r = analysis.result;
  if (r.historical_low?.price) return Math.round(r.historical_low.price);
  const prices = r.current_prices.map((p) => p.price).filter((n) => n > 0);
  if (!prices.length) return 0;
  return Math.round(Math.min(...prices) * 0.9);
}

function cheapestOffer(analysis: AnalyzeResponse) {
  const prices = [...analysis.result.current_prices].sort((a, b) => a.price - b.price);
  return prices[0] || null;
}

export function App() {
  const [items, setItems] = useState<ItemSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [analyzing, setAnalyzing] = useState(false);
  const [analysis, setAnalysis] = useState<AnalyzeResponse | null>(null);
  const [watching, setWatching] = useState(false);
  const [url, setUrl] = useState("");
  const [target, setTarget] = useState("");
  const [saving, setSaving] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const [prices, setPrices] = useState<PricePoint[]>([]);
  const [detail, setDetail] = useState<ItemSummary | null>(null);
  const bookmarkletCode = useMemo(
    () => bookmarkletFor(typeof location !== "undefined" ? location.origin : "http://localhost:4188"),
    []
  );

  const load = useCallback(async () => {
    setError("");
    try {
      const list = await api.listItems();
      setItems(list);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Kunde inte hämta listan");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    if ("serviceWorker" in navigator) {
      navigator.serviceWorker.register("/sw.js").catch(() => {});
    }
  }, [load]);

  useEffect(() => {
    if (!selected) return;
    let cancelled = false;
    (async () => {
      try {
        const fromList = items.find((i) => i.id === selected) || null;
        const pts = await api.prices(selected);
        if (!cancelled) {
          setDetail(fromList);
          setPrices(pts);
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "Detaljfel");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [selected, items]);

  async function onAnalyze(e: Event) {
    e.preventDefault();
    const q = query.trim();
    if (!q) return;
    setAnalyzing(true);
    setError("");
    setAnalysis(null);
    try {
      const res = await api.analyze(q);
      setAnalysis(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Analys misslyckades");
    } finally {
      setAnalyzing(false);
    }
  }

  async function onWatchFromAnalysis() {
    if (!analysis) return;
    const offer = cheapestOffer(analysis);
    if (!offer) {
      setError("Ingen butikslänk att bevaka");
      return;
    }
    const tp = suggestedTarget(analysis);
    setWatching(true);
    setError("");
    try {
      await api.watchFromAnalyze({
        url: offer.url,
        name: analysis.result.product.name,
        brand: analysis.result.product.brand,
        gtin: analysis.result.product.gtin,
        target_price: tp || Math.round(offer.price * 0.9),
        current_price: offer.price,
      });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Kunde inte bevaka");
    } finally {
      setWatching(false);
    }
  }

  async function onAdd(e: Event) {
    e.preventDefault();
    setSaving(true);
    setError("");
    try {
      await api.createItem(url.trim(), Number(target.replace(",", ".")));
      setUrl("");
      setTarget("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Kunde inte lägga till");
    } finally {
      setSaving(false);
    }
  }

  async function enablePush() {
    try {
      const { publicKey } = await api.vapidKey();
      if (!publicKey) {
        setError("VAPID_PUBLIC_KEY saknas på servern");
        return;
      }
      const reg = await navigator.serviceWorker.ready;
      const sub = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(publicKey),
      });
      await api.subscribePush(sub.toJSON());
      setError("");
      alert("Push aktiverat");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Push misslyckades");
    }
  }

  async function removeItem(id: string) {
    if (!confirm("Inaktivera bevakningen? Historiken behålls.")) return;
    await api.deleteItem(id);
    setSelected(null);
    await load();
  }

  if (selected && detail) {
    return (
      <div class="app-shell">
        <div class="detail-head">
          <div>
            <button class="secondary" type="button" onClick={() => setSelected(null)}>← Tillbaka</button>
            <h1 class="brand" style={{ marginTop: "0.75rem", fontSize: "1.8rem" }}>{detail.name}</h1>
            <p class="meta">
              {(() => {
                const href = safeHref(detail.url);
                return href ? (
                  <a href={href} target="_blank" rel="noreferrer noopener">{detail.store}</a>
                ) : (
                  <span>{detail.store}</span>
                );
              })()}
            </p>
          </div>
          <button class="secondary" type="button" onClick={() => removeItem(detail.id)}>Inaktivera</button>
        </div>
        <div class="panel">
          <div class="prices">
            <span>Nu: <strong>{money(detail.current_price ?? prices.at(-1)?.price, detail.currency)}</strong></span>
            <span>Mål: {money(detail.target_price, detail.currency)}</span>
            <span>Lägsta: {money(detail.lowest_price, detail.currency)}</span>
          </div>
          <PriceChart points={prices} />
        </div>
        {error && <p class="status error">{error}</p>}
      </div>
    );
  }

  const result = analysis?.result;

  return (
    <div class="app-shell">
      <header class="hero">
        <h1 class="brand">Prisläge</h1>
        <p class="tagline">Analysera när en produkt brukar vara billigast — bevaka sedan tills den når ditt mål.</p>
      </header>

      <form class="panel analyze-form" onSubmit={onAnalyze}>
        <label>
          Vad vill du köpa?
          <input
            required
            placeholder="Roborock Saros 20 Sonic robotdammsugare Vit"
            value={query}
            onInput={(e) => setQuery((e.target as HTMLInputElement).value)}
            disabled={analyzing}
          />
        </label>
        <button type="submit" disabled={analyzing}>
          {analyzing ? "Analyserar…" : "Analysera"}
        </button>
      </form>

      {analyzing && (
        <div class="panel status">Söker priser och historik via Claude (kan ta upp till en minut)…</div>
      )}

      {result && (
        <section class="panel analyze-result">
          <h2 class="analyze-headline">
            {result.product.name} är troligen billigast i {result.best_month}
          </h2>
          <div class="meta analyze-meta">
            <span class={`badge conf-${result.confidence}`}>{confidenceLabel(result.confidence)}</span>
            {analysis?.cached && <span class="badge">cache · 7 dagar</span>}
            {result.product.brand && <span>{result.product.brand}</span>}
            {result.product.variant && <span>{result.product.variant}</span>}
          </div>
          <p class="analyze-reasoning">{result.reasoning}</p>
          {result.best_period_note && <p class="meta">{result.best_period_note}</p>}

          {result.historical_low && (
            <p class="meta">
              Historiskt lågt: {money(result.historical_low.price)} ({result.historical_low.date}
              {result.historical_low.source ? ` · ${result.historical_low.source}` : ""})
            </p>
          )}
          {result.predecessor && (
            <p class="meta">
              Föregångare: {result.predecessor.name} — {result.predecessor.pattern}
            </p>
          )}

          <h3 class="section-label">Nuvarande priser</h3>
          <ul class="price-list">
            {result.current_prices.map((p) => {
              const href = safeHref(p.url);
              return (
                <li key={`${p.store}-${p.url}`}>
                  <span>{p.store}</span>
                  <strong>{money(p.price)}</strong>
                  {href ? (
                    <a href={href} target="_blank" rel="noreferrer noopener">Öppna</a>
                  ) : (
                    <span class="meta">länk saknas</span>
                  )}
                </li>
              );
            })}
          </ul>

          {result.sources?.length > 0 && (
            <>
              <h3 class="section-label">Källor</h3>
              <ul class="source-list">
                {result.sources.map((s) => {
                  const href = safeHref(s.url);
                  return (
                    <li key={s.url + s.title}>
                      {href ? (
                        <a href={href} target="_blank" rel="noreferrer noopener">{s.title}</a>
                      ) : (
                        <span>{s.title}</span>
                      )}
                    </li>
                  );
                })}
              </ul>
            </>
          )}

          <div class="toolbar" style={{ marginTop: "1rem", marginBottom: 0 }}>
            <button type="button" onClick={onWatchFromAnalysis} disabled={watching}>
              {watching
                ? "Sparar…"
                : `Bevaka · mål ${money(suggestedTarget(analysis!))}`}
            </button>
          </div>
        </section>
      )}

      {error && <p class="status error">{error}</p>}

      <div class="toolbar" style={{ marginTop: "1.75rem" }}>
        <h2 class="section-heading">Bevakningar</h2>
        <button type="button" class="secondary" onClick={load}>Uppdatera</button>
        <button type="button" class="secondary" onClick={enablePush}>Aktivera push</button>
      </div>

      <form class="panel form-row" onSubmit={onAdd}>
        <label>
          Produkt-URL
          <input
            required
            placeholder="https://www.kjell.com/..."
            value={url}
            onInput={(e) => setUrl((e.target as HTMLInputElement).value)}
          />
        </label>
        <label>
          Målpris
          <input
            required
            inputMode="decimal"
            placeholder="999"
            value={target}
            onInput={(e) => setTarget((e.target as HTMLInputElement).value)}
          />
        </label>
        <button type="submit" disabled={saving}>{saving ? "Hämtar…" : "Bevaka"}</button>
      </form>

      {loading && <p class="status">Laddar bevakningar…</p>}

      {!loading && items.length === 0 && (
        <div class="panel empty">Inga bevakningar ännu. Analysera en produkt och klicka Bevaka, eller klistra in en URL.</div>
      )}

      <div class="item-list">
        {items.map((item) => (
          <article class="item" key={item.id} onClick={() => setSelected(item.id)}>
            <div>
              <h2>{item.name}</h2>
              <div class="meta">
                {item.store}
                {item.store_mode === "capture_only" && <span class="badge capture"> capture</span>}
                {item.last_error && <span> · fel: {item.last_error}</span>}
              </div>
              <div class="prices">
                <span>Nu <strong>{money(item.current_price, item.currency)}</strong></span>
                <span>Mål {money(item.target_price, item.currency)}</span>
                <span>Lägsta {money(item.lowest_price, item.currency)}</span>
              </div>
            </div>
            <Sparkline values={item.sparkline} />
          </article>
        ))}
      </div>

      <div class="help panel">
        <strong>Bookmarklet (capture_only-butiker)</strong>
        <p>
          Bokmärket öppnar <code>/capture</code>, väntar på handskakning{" "}
          <code>prislage-ready</code> och skickar sedan JSON-LD via{" "}
          <code>postMessage</code>. Om opener blockeras (COOP) används fragment-fallback{" "}
          <code>#d=</code> (trimmat JSON-LD). Ingen fetch från butikssidan.
        </p>
        <p>
          Skapa ett bokmärke med (APP_URL = den här appens origin):
          <br />
          <code class="bookmarklet-code">{bookmarkletCode}</code>
        </p>
        <p class="meta">
          Om appen nås via Tailscale måste APP_URL vara <strong>https</strong>
          (t.ex. via <code>tailscale serve</code>), annars blockeras blandat innehåll.
        </p>
      </div>
    </div>
  );
}

const path = typeof location !== "undefined" ? location.pathname.replace(/\/$/, "") || "/" : "/";
const root = document.getElementById("app")!;
if (path === "/capture") {
  render(<CapturePage />, root);
} else {
  render(<App />, root);
}
