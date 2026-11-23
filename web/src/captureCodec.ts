/** Encode/decode capture payloads for #d= fragment fallback (never sent to server). */

export type CapturePayload = {
  type: "prislage-capture";
  url: string;
  title?: string;
  jsonld?: string;
};

const PRODUCT_TYPES = /^(Product|ProductGroup|IndividualProduct)$/i;
const OFFER_TYPES = /^(Offer|AggregateOffer)$/i;

function typeName(t: unknown): string[] {
  if (typeof t === "string") return [t];
  if (Array.isArray(t)) return t.map(String);
  return [];
}

function normType(t: string): string {
  return t.replace(/^https?:\/\/schema\.org\//i, "");
}

function isProduct(node: Record<string, unknown>): boolean {
  return typeName(node["@type"]).some((t) => PRODUCT_TYPES.test(normType(t)));
}

function isOffer(node: Record<string, unknown>): boolean {
  return typeName(node["@type"]).some((t) => OFFER_TYPES.test(normType(t)));
}

const KEEP_PRODUCT = new Set([
  "@context",
  "@type",
  "name",
  "brand",
  "sku",
  "gtin",
  "gtin8",
  "gtin12",
  "gtin13",
  "gtin14",
  "mpn",
  "productID",
  "offers",
]);

const KEEP_OFFER = new Set([
  "@type",
  "price",
  "lowPrice",
  "highPrice",
  "priceCurrency",
  "priceSpecification",
  "availability",
  "url",
  "name",
]);

function slimOffer(v: unknown): unknown {
  if (Array.isArray(v)) return v.map(slimOffer);
  if (!v || typeof v !== "object") return v;
  const node = v as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const k of Object.keys(node)) {
    if (!KEEP_OFFER.has(k)) continue;
    if (k === "priceSpecification") out[k] = slimOffer(node[k]);
    else out[k] = node[k];
  }
  return out;
}

function slimProduct(node: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const k of Object.keys(node)) {
    if (!KEEP_PRODUCT.has(k)) continue;
    if (k === "offers") out[k] = slimOffer(node[k]);
    else if (k === "brand" && node[k] && typeof node[k] === "object") {
      const b = node[k] as Record<string, unknown>;
      out[k] = { "@type": "Brand", name: b.name ?? b };
    } else out[k] = node[k];
  }
  return out;
}

/** Keep only Product/Offer nodes and drop bulky fields (images, descriptions, …). */
export function trimJSONLD(raw: string): string {
  const rawTrim = raw.trim();
  if (!rawTrim) return "";

  let parsed: unknown;
  try {
    parsed = JSON.parse(rawTrim);
  } catch {
    return "";
  }

  const products: Record<string, unknown>[] = [];

  const walk = (v: unknown) => {
    if (Array.isArray(v)) {
      v.forEach(walk);
      return;
    }
    if (!v || typeof v !== "object") return;
    const node = v as Record<string, unknown>;
    if (isProduct(node)) {
      products.push(slimProduct(node));
    } else if (isOffer(node) && products.length === 0) {
      // Standalone offer — wrap lightly for the API/parser.
      products.push({
        "@type": "Product",
        name: typeof node.name === "string" ? node.name : "",
        offers: slimOffer(node),
      });
    }
    if (node["@graph"]) walk(node["@graph"]);
    for (const key of ["item", "itemListElement", "mainEntity", "hasVariant", "offers"]) {
      if (node[key] != null) walk(node[key]);
    }
  };
  walk(parsed);

  if (products.length === 0) return "";
  if (products.length === 1) return JSON.stringify(products[0]);
  return JSON.stringify(products);
}

export function encodeFragmentPayload(payload: CapturePayload): string {
  const trimmed: CapturePayload = {
    ...payload,
    jsonld: payload.jsonld ? trimJSONLD(payload.jsonld) : "",
  };
  const json = JSON.stringify(trimmed);
  const bytes = new TextEncoder().encode(json);
  let bin = "";
  bytes.forEach((b) => {
    bin += String.fromCharCode(b);
  });
  return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

export function decodeFragmentPayload(fragment: string): CapturePayload | null {
  let b64 = fragment;
  if (b64.startsWith("#d=")) b64 = b64.slice(3);
  else if (b64.startsWith("d=")) b64 = b64.slice(2);
  if (!b64) return null;
  try {
    const pad = "=".repeat((4 - (b64.length % 4)) % 4);
    const bin = atob(b64.replace(/-/g, "+").replace(/_/g, "/") + pad);
    const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0));
    const json = new TextDecoder().decode(bytes);
    const data = JSON.parse(json) as CapturePayload;
    if (!data || data.type !== "prislage-capture" || !data.url) return null;
    return data;
  } catch {
    return null;
  }
}
