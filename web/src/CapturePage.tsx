import { useEffect, useMemo, useState } from "preact/hooks";
import { api, money } from "./api";
import { previewFromCapture, type CapturePreview } from "./preview";

type CapturePayload = {
  type: "prislage-capture";
  url: string;
  title?: string;
  jsonld?: string;
};

export function CapturePage() {
  const [payload, setPayload] = useState<CapturePayload | null>(null);
  const [status, setStatus] = useState<"waiting" | "ready" | "saving" | "saved" | "error">("waiting");
  const [error, setError] = useState("");
  const [target, setTarget] = useState("");
  const [savedName, setSavedName] = useState("");

  const preview: CapturePreview | null = useMemo(() => {
    if (!payload) return null;
    return previewFromCapture({
      url: payload.url,
      title: payload.title,
      jsonld: payload.jsonld,
    });
  }, [payload]);

  useEffect(() => {
    if (!window.opener) {
      setStatus("error");
      setError("Öppna den här sidan via bookmarkleten (popup från butikssidan).");
      return;
    }

    function onMessage(ev: MessageEvent) {
      if (!window.opener || ev.source !== window.opener) return;
      const data = ev.data as CapturePayload | null;
      if (!data || data.type !== "prislage-capture" || !data.url) return;

      let productOrigin = "";
      try {
        productOrigin = new URL(data.url).origin;
      } catch {
        return;
      }
      if (ev.origin !== productOrigin) {
        setStatus("error");
        setError(`Otillåten origin: ${ev.origin} matchar inte produkt-URL.`);
        return;
      }

      setPayload(data);
      setStatus("ready");
      setError("");

      // Ack so bookmarklet stops retrying.
      try {
        (ev.source as Window).postMessage({ type: "prislage-capture-ack" }, ev.origin);
      } catch {
        /* ignore */
      }
    }

    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, []);

  useEffect(() => {
    if (preview?.price && !target) {
      setTarget(preview.price);
    }
  }, [preview, target]);

  async function onSave() {
    if (!payload) return;
    setStatus("saving");
    setError("");
    try {
      const body: Record<string, unknown> = {
        url: payload.url,
        jsonld: payload.jsonld || undefined,
      };
      if (!payload.jsonld) {
        throw new Error("Ingen JSON-LD hittades på sidan — kan inte spara.");
      }
      const tp = Number(String(target).replace(",", "."));
      if (!Number.isNaN(tp) && tp > 0) {
        body.target_price = tp;
      }
      const item = await api.capture(body);
      setSavedName(item.name);
      setStatus("saved");
    } catch (e) {
      setStatus("error");
      setError(e instanceof Error ? e.message : "Kunde inte spara");
    }
  }

  return (
    <div class="app-shell">
      <h1 class="brand" style={{ fontSize: "1.9rem" }}>Spara till Prisläge</h1>
      <p class="tagline">Förhandsvisa produkten från butikssidan och spara när det ser rätt ut.</p>

      {status === "waiting" && (
        <div class="panel status">Väntar på data från bookmarkleten…</div>
      )}

      {preview && (status === "ready" || status === "saving" || status === "error") && (
        <div class="panel capture-preview">
          <div class="meta">{preview.store}</div>
          <h2 style={{ fontFamily: "var(--font-display)", margin: "0.35rem 0" }}>{preview.name}</h2>
          {preview.brand && <div class="meta">Varumärke: {preview.brand}</div>}
          <div class="prices" style={{ marginTop: "0.75rem" }}>
            <span>
              Pris{" "}
              <strong>
                {preview.price ? money(preview.price, preview.currency) : "okänt"}
              </strong>
            </span>
          </div>
          <label style={{ marginTop: "1rem" }}>
            Målpris
            <input
              inputMode="decimal"
              value={target}
              onInput={(e) => setTarget((e.target as HTMLInputElement).value)}
            />
          </label>
          <div class="toolbar" style={{ marginTop: "1rem", marginBottom: 0 }}>
            <button type="button" onClick={onSave} disabled={status === "saving"}>
              {status === "saving" ? "Sparar…" : "Spara"}
            </button>
            <button type="button" class="secondary" onClick={() => window.close()}>
              Avbryt
            </button>
          </div>
        </div>
      )}

      {status === "saved" && (
        <div class="panel">
          <p>
            Sparat: <strong>{savedName}</strong>
          </p>
          <div class="toolbar">
            <a class="btn" href="/">
              Öppna Prisläge
            </a>
            <button type="button" class="secondary" onClick={() => window.close()}>
              Stäng
            </button>
          </div>
        </div>
      )}

      {error && <p class="status error">{error}</p>}
    </div>
  );
}
