# Prisläge

Analysera när en produkt troligen är billigast (Claude + webbsökning), bevaka sedan tills priset når ditt mål. Capture/bookmarklet fyller egen historik som gör framtida analyser bättre.

> **MVP-notis:** Ingen användarinloggning. Appen är tänkt att köras privat (t.ex. bakom Tailscale) för en användare. API:t skyddas med en statisk token (`API_TOKEN`).

## Hybridmodell

| Läge | Betydelse |
|------|-----------|
| `auto` | Schemaläggaren hämtar produktsidan med PrislageBot |
| `capture_only` | Ingen automatisk fetch — spara via bookmarklet → `/capture` |

Seedade butiker:

- **auto:** `kjell.com`, `apple.com`
- **capture_only:** `elgiganten.se`, `inet.se`, `netonnet.se`, `webhallen.com`, `komplett.se`

Nya okända domäner läggs in som `capture_only`. Vi ändrar aldrig User-Agent för att komma förbi blockering.

GTIN sparas bara om det är 8/12/13/14 siffror med giltig GS1-kontrollsiffra; annars går värdet till `sku`.

## Kör lokalt med Docker

```bash
cp .env.example .env
docker compose up --build
```

- API: http://localhost:3847  
- Web: http://localhost:4188  
- Postgres: localhost:5433  

Token default: `dev-token-change-me` (sätt `API_TOKEN` i `.env`).

Sätt `ANTHROPIC_API_KEY` för Analysera (`POST /api/analyze`). Nyckeln stannar på API-servern. Modell default: `claude-sonnet-4-5` (override med `ANTHROPIC_MODEL`). Webbsökning via Anthropics serververktyg `web_search` (`type: web_search_20250305`).

Sätt `CORS_ORIGIN` till webappens origin (default `http://localhost:4188`). Butiksdomäner får **inte** anropa API:t.

### Utan Docker (utveckling)

```bash
# DB
docker compose up -d db
export DATABASE_URL=postgres://prislage:prislage@localhost:5433/prislage?sslmode=disable
export API_TOKEN=dev-token-change-me
export CORS_ORIGIN=http://localhost:4188

cd api
go run ./cmd/prislage migrate
go run ./cmd/server

# annan terminal
cd web
npm install
VITE_API_BASE=http://localhost:3847 VITE_API_TOKEN=dev-token-change-me npm run dev
```

## CLI

```bash
cd api
go run ./cmd/prislage parse -json 'https://www.kjell.com/...'
go run ./cmd/prislage fetch          # CronJob / manuell dygnskörning (bara auto-butiker)
go run ./cmd/prislage migrate
```

## API (Bearer token)

| Metod | Path | Beskrivning |
|-------|------|-------------|
| POST | `/api/analyze` | `{ query }` → Claude + web_search; cachas 7 dagar i `analyses` |
| POST | `/api/watch` | skapa bevakning utan live-fetch (från analysresultat) |
| GET | `/api/items` | lista bevakningar |
| POST | `/api/items` | `{ url, target_price }` — hämtar namn/pris (bara auto) |
| GET/PATCH/DELETE | `/api/items/{id}` | detalj / uppdatera / soft delete (`active=false`) |
| GET | `/api/items/{id}/prices` | historik |
| POST | `/api/capture` | `{ url, jsonld }` → price_point `source=capture` (anropas från PWA, inte från butiken) |
| GET | `/api/stores` | butiksläge |
| POST/DELETE | `/api/push/subscribe` | Web Push |

CORS är en allowlist (`CORS_ORIGIN`, kommaseparerad). Tom lista / `*` ger **ingen** cross-origin-access — använd same-origin bakom reverse proxy i produktion.

## Bookmarklet (CSP-säker)

Butiker blockerar ofta `connect-src` (ingen `fetch` till ditt API) och ibland `script-src`. Flödet är därför:

1. Bookmarkleten (inline `javascript:`) samlar `url`, `document.title` och trimmar JSON-LD till Product/Offer-noder (~8 KB).
2. Den öppnar `{APP_URL}/capture` i ett nytt fönster och **väntar** på handskakning `{type:'prislage-ready'}` från capture-sidan.
3. Först då skickas datan med `postMessage` (targetOrigin = APP_URL). Timeout **5 s**.
4. Capture-sidan kontrollerar att `event.source === window.opener` och att `event.origin` matchar **produkt-URL:ens** origin, visar förhandsvisning och POSTar till `/api/capture` med token när du klickar **Spara**.

### COOP / opener saknas (fragment-fallback)

Om butiken sätter Cross-Origin-Opener-Policy så att `window.opener` är `null` (eller postMessage aldrig kommer fram) navigar bookmarkleten efter timeout till:

`{APP_URL}/capture#d=<base64url(JSON)>`

Fragmentet skickas **aldrig** till servern. Capture-sidan avkodar `#d=`, visar förhandsvisning med badge **källa ej verifierad** (ingen origin att kontrollera) och sparar bara när du klickar **Spara**.

Kopiera bokmärkes-URL:en från hjälptexten i PWA:n (den bakas med aktuell `location.origin` som `APP_URL`).

Referensimplementation: `web/public/capture.js` / `web/src/bookmarklet.ts` (samma protokoll; använd hellre den inbyggda `javascript:`-länken så ingen extern script-load behövs från butikssidan).

### HTTPS / Tailscale

Om du når appen via Tailscale (eller annan tunnling) **måste `APP_URL` vara https** (t.ex. `tailscale serve` / HTTPS-proxy). Annars blockerar webbläsaren bookmarkleten som **mixed content** när butikssidan är https och capture-fönstret skulle öppnas som http.

## Tester

```bash
cd api
go test ./...
```

## Miljövariabler

Se `.env.example`. Viktiga: `DATABASE_URL`, `API_TOKEN`, `CORS_ORIGIN`, `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`.
