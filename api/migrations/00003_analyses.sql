-- +goose Up
CREATE TABLE analyses (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    query       TEXT NOT NULL,
    query_key   TEXT NOT NULL,
    result      JSONB NOT NULL,
    cached      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX analyses_query_key_created_at_idx ON analyses (query_key, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS analyses;
