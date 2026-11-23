import { render } from "preact";
import { useCallback, useEffect, useMemo, useState } from "preact/hooks";
import { api, money, type ItemSummary, type PricePoint } from "./api";
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

export function App() {
  const [items, setItems] = useState<ItemSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
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
            <p class="meta"><a href={detail.url} target="_blank" rel="noreferrer">{detail.store}</a></p>
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

  return (
    <div class="app-shell">
      <h1 class="brand">Prisläge</h1>
      <p class="tagline">Bevaka produkter, spara historik från dag ett och få larm när priset når ditt mål.</p>

      <div class="toolbar">
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
      {error && <p class="status error">{error}</p>}

      {!loading && items.length === 0 && (
        <div class="panel empty">Inga bevakningar ännu. Klistra in en URL från en auto-butik (t.ex. Kjell) eller använd bookmarkleten för capture_only-butiker.</div>
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
          Bokmärket öppnar <code>/capture</code> och skickar JSON-LD via{" "}
          <code>postMessage</code> — ingen fetch från butikssidan (undviker CSP{" "}
          <code>connect-src</code>).
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

function bookmarkletFor(appOrigin: string): string {
  // Self-contained: no external script load (script-src CSP on stores).
  const app = JSON.stringify(appOrigin.replace(/\/$/, ""));
  return `javascript:(function(){var A=${app};var O=new URL(A).origin;var c=[].slice.call(document.querySelectorAll('script[type="application/ld+json"]')).map(function(e){return(e.textContent||"").trim()}).filter(Boolean);var j="";if(c.length===1)j=c[0];else if(c.length>1)j=JSON.stringify(c.map(function(x){try{return JSON.parse(x)}catch(e){return x}}));var p={type:"prislage-capture",url:location.href,title:document.title||"",jsonld:j};var w=window.open(A+"/capture","prislage_capture");if(!w){alert("Popup blocker?");return}var n=0,t=setInterval(function(){if(!w||w.closed){clearInterval(t);return}try{w.postMessage(p,O)}catch(e){}if(++n>40)clearInterval(t)},150);function ack(ev){if(ev.origin!==O||!ev.data||ev.data.type!=="prislage-capture-ack")return;clearInterval(t);window.removeEventListener("message",ack)}window.addEventListener("message",ack)})();`;
}

const path = typeof location !== "undefined" ? location.pathname.replace(/\/$/, "") || "/" : "/";
const root = document.getElementById("app")!;
if (path === "/capture") {
  render(<CapturePage />, root);
} else {
  render(<App />, root);
}
