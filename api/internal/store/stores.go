package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"prislage/internal/models"
	"prislage/internal/urlx"
)

var ErrUnknownStore = errors.New("unknown store")

type Stores struct {
	pool *pgxpool.Pool
}

func NewStores(pool *pgxpool.Pool) *Stores {
	return &Stores{pool: pool}
}

func (s *Stores) Get(ctx context.Context, domain string) (*models.Store, error) {
	var st models.Store
	err := s.pool.QueryRow(ctx,
		`SELECT domain, mode, notes FROM stores WHERE domain = $1`, domain,
	).Scan(&st.Domain, &st.Mode, &st.Notes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknownStore
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Stores) UpsertDefault(ctx context.Context, domain string) (*models.Store, error) {
	if domain == "" {
		return nil, fmt.Errorf("empty store domain")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO stores (domain, mode, notes)
		VALUES ($1, 'capture_only', 'auto-added; not yet verified for auto fetch')
		ON CONFLICT (domain) DO NOTHING
	`, domain)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, domain)
}

func (s *Stores) ModeForURL(ctx context.Context, rawURL string) (models.StoreMode, string, error) {
	domain := urlx.StoreHost(rawURL)
	st, err := s.Get(ctx, domain)
	if errors.Is(err, ErrUnknownStore) {
		st, err = s.UpsertDefault(ctx, domain)
	}
	if err != nil {
		return "", domain, err
	}
	return st.Mode, domain, nil
}

func (s *Stores) List(ctx context.Context) ([]models.Store, error) {
	rows, err := s.pool.Query(ctx, `SELECT domain, mode, notes FROM stores ORDER BY domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Store
	for rows.Next() {
		var st models.Store
		if err := rows.Scan(&st.Domain, &st.Mode, &st.Notes); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}
