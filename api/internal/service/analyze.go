package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"prislage/internal/analyze"
	"prislage/internal/models"
	"prislage/internal/store"
	"prislage/internal/urlx"
)

const analyzeCacheTTL = 7 * 24 * time.Hour

type AnalyzeResponse struct {
	ID        string          `json:"id"`
	Query     string          `json:"query"`
	Cached    bool            `json:"cached"`
	CreatedAt time.Time       `json:"created_at"`
	Result    analyze.Result  `json:"result"`
}

func (s *Service) Analyze(ctx context.Context, query string) (*AnalyzeResponse, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query required")
	}
	if s.Claude == nil {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY saknas")
	}
	key := analyze.NormalizeQueryKey(query)

	if row, err := s.Analyses.FindFresh(ctx, key, analyzeCacheTTL); err == nil {
		var res analyze.Result
		if err := json.Unmarshal(row.Result, &res); err != nil {
			return nil, err
		}
		return &AnalyzeResponse{
			ID: row.ID, Query: row.Query, Cached: true, CreatedAt: row.CreatedAt, Result: res,
		}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	watched := s.watchedContext(ctx)
	res, err := s.Claude.Analyze(ctx, query, watched)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	row, err := s.Analyses.Insert(ctx, query, key, raw)
	if err != nil {
		return nil, err
	}
	return &AnalyzeResponse{
		ID: row.ID, Query: row.Query, Cached: false, CreatedAt: row.CreatedAt, Result: *res,
	}, nil
}

func (s *Service) watchedContext(ctx context.Context) []map[string]any {
	items, err := s.Items.ListActive(ctx)
	if err != nil || len(items) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		m := map[string]any{
			"name":  it.Name,
			"store": it.Store,
			"url":   it.URL,
			"brand": deref(it.Brand),
		}
		if it.CurrentPrice != nil {
			m["current_price"], _ = it.CurrentPrice.Float64()
		}
		if it.LowestPrice != nil {
			m["lowest_price"], _ = it.LowestPrice.Float64()
		}
		if it.GTIN != nil {
			m["gtin"] = *it.GTIN
		}
		out = append(out, m)
	}
	return out
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

type WatchFromAnalyzeInput struct {
	URL          string
	Name         string
	Brand        string
	GTIN         string
	TargetPrice  decimal.Decimal
	CurrentPrice decimal.Decimal
}

// WatchFromAnalyze creates a watchlist item without live fetch (works for capture_only stores).
func (s *Service) WatchFromAnalyze(ctx context.Context, in WatchFromAnalyzeInput) (*store.ItemSummary, error) {
	norm, err := urlx.Normalize(in.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if existing, err := s.Items.GetByNormalizedURL(ctx, norm); err == nil {
		if in.TargetPrice.IsPositive() {
			_, _ = s.Items.Update(ctx, existing.ID, store.UpdateItemParams{TargetPrice: &in.TargetPrice})
		}
		return s.summaryOf(ctx, existing.ID)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	_, domain, err := s.Stores.ModeForURL(ctx, norm)
	if err != nil {
		return nil, err
	}
	name := nonEmpty(in.Name, domain)
	target := in.TargetPrice
	if !target.IsPositive() && in.CurrentPrice.IsPositive() {
		target = in.CurrentPrice.Mul(decimal.NewFromFloat(0.9)).Round(0)
	}
	if !target.IsPositive() {
		return nil, fmt.Errorf("target_price required")
	}

	item, err := s.Items.Create(ctx, store.CreateItemParams{
		Name:          name,
		URL:           in.URL,
		NormalizedURL: norm,
		Store:         domain,
		GTIN:          strPtr(in.GTIN),
		Brand:         strPtr(in.Brand),
		TargetPrice:   target,
		Currency:      "SEK",
	})
	if err != nil {
		return nil, err
	}
	price := in.CurrentPrice
	if !price.IsPositive() {
		price = target
	}
	if _, err := s.recordPrice(ctx, item, price, "SEK", nil, models.SourceManual); err != nil {
		return nil, err
	}
	_ = s.Items.MarkFetchOK(ctx, item.ID, name, in.Brand, strPtr(in.GTIN), nil)
	return s.summaryOf(ctx, item.ID)
}
