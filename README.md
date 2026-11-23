# Prisläge

Prisbevakning: klistra in en produkt-URL, sätt målpris, spara historik från dag ett och få larm när priset når dit.

> **MVP-notis:** Ingen inloggning. Appen är tänkt att köras privat (t.ex. bakom Tailscale) för en användare.

## Status

Fas 1 under uppbyggnad. Just nu finns:

- Postgres-migrationer (`api/migrations/`)
- Generisk prisparser (JSON-LD → Open Graph) + `Fetcher`-interface
- CLI `prislage parse` för att testa mot riktiga butikssidor
- Enhetstester med sparade HTML-exempel i `api/testdata/`

API, schemaläggare, frontend och docker-compose kommer i nästa steg.

## Förutsättningar

- Go 1.22+

## Testa parsningen (enhetstester)

```bash
cd api
go test ./internal/urlx ./internal/fetch -count=1
```

## Testa mot en riktig butik

```bash
cd api
go run ./cmd/prislage parse 'https://exempelbutik.se/produkt/...'
```

JSON-utdata:

```bash
go run ./cmd/prislage parse -json 'https://exempelbutik.se/produkt/...'
```

Offline / regression mot sparad HTML:

```bash
go run ./cmd/prislage parse -file testdata/product_jsonld.html -url 'https://shop.example/p/kaffe'
```

Parsers prioritetsordning:

1. `application/ld+json` med schema.org `Product` / `Offer` (inkl. `@graph` och `AggregateOffer.lowPrice`)
2. Open Graph / product meta-taggar (`product:price:amount`, `og:price:amount`, …)

Extrafält från JSON-LD när de finns: `brand`, `gtin` / `gtin13` m.fl.

## Mappstruktur (utdrag)

```text
api/
  cmd/prislage/          # CLI
  internal/fetch/        # Fetcher + JSON-LD/OG
  internal/urlx/         # URL-normalisering
  internal/models/       # Domänmodeller
  migrations/            # goose SQL
  testdata/              # HTML-fixtures
```

## Datamodell (migration 00001)

- `items` — inkl. `normalized_url`, `gtin`, `brand`, `last_fetched_at`, `last_error`, `fail_count`; partiellt unikt index på `normalized_url WHERE active`
- `price_points` — inkl. `source` (`jsonld` | `og` | `manual`); index `(item_id, fetched_at DESC)`
- `notifications` — `target_reached` / `new_low`
- `push_subscriptions` — Web Push (senare)

Soft delete: `DELETE` sätter `active = false` (kommer i API-steget). Historik behålls.

### Larm-dedupe (kommer i fetch-pipelinen)

- `target_reached` bara när priset *korsar* målet (föregående > target, nytt ≤ target)
- `new_low` bara om nytt pris är ≥ 1 % lägre än tidigare lägsta
