package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Analyses struct {
	pool *pgxpool.Pool
}

func NewAnalyses(pool *pgxpool.Pool) *Analyses {
	return &Analyses{pool: pool}
}

type AnalysisRow struct {
	ID        string          `json:"id"`
	Query     string          `json:"query"`
	QueryKey  string          `json:"query_key"`
	Result    json.RawMessage `json:"result"`
	Cached    bool            `json:"cached"`
	CreatedAt time.Time       `json:"created_at"`
}

func (a *Analyses) FindFresh(ctx context.Context, queryKey string, maxAge time.Duration) (*AnalysisRow, error) {
	cutoff := time.Now().UTC().Add(-maxAge)
	row := a.pool.QueryRow(ctx, `
		SELECT id, query, query_key, result, cached, created_at
		FROM analyses
		WHERE query_key = $1 AND created_at > $2
		ORDER BY created_at DESC
		LIMIT 1
	`, queryKey, cutoff)
	var out AnalysisRow
	err := row.Scan(&out.ID, &out.Query, &out.QueryKey, &out.Result, &out.Cached, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pgx.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *Analyses) Insert(ctx context.Context, query, queryKey string, result json.RawMessage) (*AnalysisRow, error) {
	row := a.pool.QueryRow(ctx, `
		INSERT INTO analyses (query, query_key, result, cached)
		VALUES ($1, $2, $3, FALSE)
		RETURNING id, query, query_key, result, cached, created_at
	`, query, queryKey, result)
	var out AnalysisRow
	if err := row.Scan(&out.ID, &out.Query, &out.QueryKey, &out.Result, &out.Cached, &out.CreatedAt); err != nil {
		return nil, err
	}
	return &out, nil
}
