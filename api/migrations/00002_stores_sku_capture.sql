-- +goose Up
ALTER TABLE items ADD COLUMN sku TEXT;

ALTER TABLE price_points DROP CONSTRAINT IF EXISTS price_points_source_check;
ALTER TABLE price_points ADD CONSTRAINT price_points_source_check
    CHECK (source IN ('jsonld', 'og', 'manual', 'capture'));

CREATE TABLE stores (
    domain  TEXT PRIMARY KEY,
    mode    TEXT NOT NULL CHECK (mode IN ('auto', 'capture_only')),
    notes   TEXT NOT NULL DEFAULT ''
);

INSERT INTO stores (domain, mode, notes) VALUES
    ('kjell.com',       'auto',          'JSON-LD fungerar med PrislageBot'),
    ('apple.com',       'auto',          'JSON-LD fungerar med PrislageBot'),
    ('elgiganten.se',   'capture_only',  'Vercel Security Checkpoint — använd bookmarklet'),
    ('inet.se',         'capture_only',  'Cloudflare — använd bookmarklet'),
    ('netonnet.se',     'capture_only',  'Blockerar bot-UA; HTML-capture fungerar'),
    ('webhallen.com',   'capture_only',  'JS-renderad SPA — använd bookmarklet'),
    ('komplett.se',     'capture_only',  'Otillgänglig / timeout från datacenter');

-- +goose Down
DELETE FROM stores;
DROP TABLE IF EXISTS stores;

ALTER TABLE price_points DROP CONSTRAINT IF EXISTS price_points_source_check;
ALTER TABLE price_points ADD CONSTRAINT price_points_source_check
    CHECK (source IN ('jsonld', 'og', 'manual'));

ALTER TABLE items DROP COLUMN IF EXISTS sku;
