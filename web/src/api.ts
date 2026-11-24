const API_BASE = (import.meta.env.VITE_API_BASE as string) || "http://localhost:3847";
const API_TOKEN = (import.meta.env.VITE_API_TOKEN as string) || "dev-token-change-me";

export type ItemSummary = {
  id: string;
  name: string;
  url: string;
  store: string;
  store_mode?: string;
  target_price: string;
  currency: string;
  current_price?: string;
  lowest_price?: string;
  sparkline?: number[];
  gtin?: string;
  sku?: string;
  brand?: string;
  last_error?: string;
  last_fetched_at?: string;
};

export type PricePoint = {
  id: string;
  price: string;
  currency: string;
  source: string;
  fetched_at: string;
  in_stock?: boolean;
};

export type AnalyzeResult = {
  product: {
    name: string;
    brand: string;
    variant: string;
    gtin?: string;
  };
  current_prices: { store: string; price: number; url: string }[];
  historical_low?: { price: number; date: string; source: string };
  predecessor?: { name: string; pattern: string };
  best_month: string;
  best_period_note: string;
  confidence: "low" | "medium" | "high";
  reasoning: string;
  sources: { title: string; url: string }[];
};

export type AnalyzeResponse = {
  id: string;
  query: string;
  cached: boolean;
  created_at: string;
  result: AnalyzeResult;
};

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Authorization", `Bearer ${API_TOKEN}`);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const res = await fetch(`${API_BASE}${path}`, { ...init, headers });
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error((data as { error?: string }).error || res.statusText);
  }
  return data as T;
}

export const api = {
  base: API_BASE,
  token: API_TOKEN,
  listItems: () => request<ItemSummary[]>("/api/items"),
  createItem: (url: string, target_price: number) =>
    request<ItemSummary>("/api/items", {
      method: "POST",
      body: JSON.stringify({ url, target_price }),
    }),
  watchFromAnalyze: (body: {
    url: string;
    name?: string;
    brand?: string;
    gtin?: string;
    target_price: number;
    current_price?: number;
  }) =>
    request<ItemSummary>("/api/watch", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  analyze: (query: string) =>
    request<AnalyzeResponse>("/api/analyze", {
      method: "POST",
      body: JSON.stringify({ query }),
    }),
  getItem: (id: string) => request<ItemSummary>(`/api/items/${id}`),
  deleteItem: (id: string) => request<void>(`/api/items/${id}`, { method: "DELETE" }),
  patchItem: (id: string, body: Record<string, unknown>) =>
    request<ItemSummary>(`/api/items/${id}`, { method: "PATCH", body: JSON.stringify(body) }),
  prices: (id: string) => request<PricePoint[]>(`/api/items/${id}/prices`),
  vapidKey: () => request<{ publicKey: string }>("/api/vapid-public-key"),
  subscribePush: (sub: PushSubscriptionJSON) =>
    request<{ status: string }>("/api/push/subscribe", {
      method: "POST",
      body: JSON.stringify(sub),
    }),
  capture: (body: { url: string; jsonld?: string; html?: string; target_price?: number }) =>
    request<ItemSummary>("/api/capture", {
      method: "POST",
      body: JSON.stringify(body),
    }),
};

export function money(value?: string | number | null, currency = "SEK"): string {
  if (value == null || value === "") return "—";
  const n = typeof value === "number" ? value : Number(value);
  if (Number.isNaN(n)) return String(value);
  return new Intl.NumberFormat("sv-SE", { style: "currency", currency, maximumFractionDigits: 0 }).format(n);
}

/** Only allow http(s) links in the UI. */
export function safeHref(raw?: string | null): string | null {
  if (!raw) return null;
  try {
    const u = new URL(raw);
    if (u.protocol === "http:" || u.protocol === "https:") return u.href;
  } catch {
    /* ignore */
  }
  return null;
}
