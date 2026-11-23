-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    url             TEXT NOT NULL,
    normalized_url  TEXT NOT NULL,
    store           TEXT NOT NULL DEFAULT '',
    category        TEXT NOT NULL DEFAULT '',
    gtin            TEXT,
    brand           TEXT,
    target_price    NUMERIC(12, 2) NOT NULL,
    currency        CHAR(3) NOT NULL DEFAULT 'SEK',
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    last_fetched_at TIMESTAMPTZ,
    last_error      TEXT,
    fail_count      INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One active watch per normalized product URL; inactive rows keep history.
CREATE UNIQUE INDEX items_normalized_url_active_uidx
    ON items (normalized_url)
    WHERE active = TRUE;

CREATE INDEX items_active_idx ON items (active) WHERE active = TRUE;

CREATE TABLE price_points (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id    UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    price      NUMERIC(12, 2) NOT NULL,
    currency   CHAR(3) NOT NULL,
    in_stock   BOOLEAN,
    source     TEXT NOT NULL CHECK (source IN ('jsonld', 'og', 'manual')),
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX price_points_item_fetched_idx
    ON price_points (item_id, fetched_at DESC);

CREATE TABLE notifications (
    id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    type    TEXT NOT NULL CHECK (type IN ('target_reached', 'new_low')),
    price   NUMERIC(12, 2) NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX notifications_item_sent_idx
    ON notifications (item_id, sent_at DESC);

CREATE TABLE push_subscriptions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint   TEXT NOT NULL UNIQUE,
    p256dh     TEXT NOT NULL,
    auth       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS push_subscriptions;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS price_points;
DROP TABLE IF EXISTS items;
