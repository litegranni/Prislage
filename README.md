# Prisläge

Prisbevakning: klistra in en produkt-URL, sätt målpris, spara historik från dag ett och få larm när priset når dit.

> **MVP-notis:** Ingen användarinloggning. Appen är tänkt att köras privat (t.ex. bakom Tailscale) för en användare. API:t skyddas med en statisk token (`API_TOKEN`).

## Hybridmodell

| Läge | Betydelse |
|------|-----------|
| `auto` | Schemaläggaren hämtar produktsidan med PrislageBot |
| `capture_only` | Ingen automatisk fetch — spara via bookmarklet / `POST /api/capture` |

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

### Utan Docker (utveckling)

```bash
# DB
docker compose up -d db
export DATABASE_URL=postgres://prislage:prislage@localhost:5433/prislage?sslmode=disable
export API_TOKEN=dev-token-change-me

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
| GET | `/api/items` | lista bevakningar |
| POST | `/api/items` | `{ url, target_price }` — hämtar namn/pris (bara auto) |
| GET/PATCH/DELETE | `/api/items/{id}` | detalj / uppdatera / soft delete (`active=false`) |
| GET | `/api/items/{id}/prices` | historik |
| POST | `/api/capture` | `{ url, html }` eller `{ url, jsonld }` → price_point `source=capture` |
| GET | `/api/stores` | butiksläge |
| POST/DELETE | `/api/push/subscribe` | Web Push |

## Bookmarklet

1. Öppna web-appen och kör i konsolen:

```js
localStorage.setItem('prislage_api_base', 'http://localhost:3847');
localStorage.setItem('prislage_api_token', 'dev-token-change-me');
```

2. Skapa ett bokmärke med URL:

```text
javascript:(function(){var s=document.createElement('script');s.src='http://localhost:4188/capture.js';document.body.appendChild(s);})();
```

3. På en produktsida: klicka bokmärket. Det läser `application/ld+json` (fallback: hela HTML) och POSTar till `/api/capture`.

Skriptet ligger i `web/public/capture.js`.

## Tester

```bash
cd api
go test ./...
```

## Miljövariabler

Se `.env.example`. Viktiga: `DATABASE_URL`, `API_TOKEN`, `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`.
