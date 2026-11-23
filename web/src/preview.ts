/** Lightweight JSON-LD preview for the capture page (browser-side only). */

export type CapturePreview = {
  name: string;
  price: string | null;
  currency: string;
  store: string;
  brand?: string;
};

export function storeHost(rawURL: string): string {
  try {
    return new URL(rawURL).hostname.replace(/^www\./, "");
  } catch {
    return "";
  }
}

export function previewFromCapture(input: {
  url: string;
  title?: string;
  jsonld?: string;
}): CapturePreview {
  const store = storeHost(input.url);
  const fallback: CapturePreview = {
    name: (input.title || "").trim() || store || "Produkt",
    price: null,
    currency: "SEK",
    store,
  };

  const raw = (input.jsonld || "").trim();
  if (!raw) return fallback;

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return fallback;
  }

  const nodes = flattenJSONLD(parsed);
  for (const node of nodes) {
    if (!isProductLike(node)) continue;
    const name = stringField(node, "name") || fallback.name;
    const brand = brandField(node);
    const offer = firstOffer(node);
    if (!offer) {
      return { ...fallback, name, brand };
    }
    const price = priceFromOffer(offer);
    const currency = (stringField(offer, "priceCurrency") || "SEK").toUpperCase();
    return {
      name,
      price,
      currency,
      store,
      brand,
    };
  }

  return fallback;
}

function flattenJSONLD(v: unknown, out: Record<string, unknown>[] = []): Record<string, unknown>[] {
  if (Array.isArray(v)) {
    for (const el of v) flattenJSONLD(el, out);
    return out;
  }
  if (v && typeof v === "object") {
    const obj = v as Record<string, unknown>;
    out.push(obj);
    if (obj["@graph"]) flattenJSONLD(obj["@graph"], out);
    for (const key of ["item", "itemListElement", "mainEntity", "hasVariant", "offers"]) {
      if (obj[key] != null) flattenJSONLD(obj[key], out);
    }
  }
  return out;
}

function typeList(node: Record<string, unknown>): string[] {
  const t = node["@type"];
  if (typeof t === "string") return [t];
  if (Array.isArray(t)) return t.map(String);
  return [];
}

function isProductLike(node: Record<string, unknown>): boolean {
  return typeList(node).some((t) => {
    const s = t.replace(/^https?:\/\/schema\.org\//, "");
    return /^(Product|ProductGroup|IndividualProduct)$/i.test(s);
  });
}

function stringField(node: Record<string, unknown>, key: string): string {
  const v = node[key];
  if (typeof v === "string" || typeof v === "number") return String(v).trim();
  return "";
}

function brandField(node: Record<string, unknown>): string | undefined {
  const b = node.brand;
  if (typeof b === "string") return b;
  if (b && typeof b === "object" && "name" in b) return String((b as { name: unknown }).name);
  return undefined;
}

function firstOffer(node: Record<string, unknown>): Record<string, unknown> | null {
  const offers = node.offers;
  if (!offers) return null;
  if (Array.isArray(offers)) {
    const first = offers[0];
    return first && typeof first === "object" ? (first as Record<string, unknown>) : null;
  }
  if (typeof offers === "object") return offers as Record<string, unknown>;
  return null;
}

function priceFromOffer(offer: Record<string, unknown>): string | null {
  const direct = stringField(offer, "price") || stringField(offer, "lowPrice");
  if (direct) return normalizePrice(direct);
  const spec = offer.priceSpecification;
  const specs = Array.isArray(spec) ? spec : spec ? [spec] : [];
  for (const s of specs) {
    if (!s || typeof s !== "object") continue;
    const obj = s as Record<string, unknown>;
    const priceType = String(obj.priceType || "");
    if (/StrikethroughPrice/i.test(priceType)) continue;
    const p = stringField(obj, "price");
    if (p) return normalizePrice(p);
  }
  return null;
}

function normalizePrice(raw: string): string {
  let s = raw.replace(/\u00a0/g, " ").trim();
  s = s.replace(/(kr|SEK|€|EUR|\$|USD)/gi, "").replace(/\s/g, "");
  if (s.includes(",") && s.includes(".")) {
    s = s.replace(/\./g, "").replace(",", ".");
  } else if (s.includes(",")) {
    s = s.replace(",", ".");
  }
  return s;
}
