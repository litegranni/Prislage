package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"prislage/internal/models"
)

type Items struct {
	pool *pgxpool.Pool
}

func NewItems(pool *pgxpool.Pool) *Items {
	return &Items{pool: pool}
}

type ItemSummary struct {
	models.Item
	CurrentPrice *decimal.Decimal `json:"current_price,omitempty"`
	LowestPrice  *decimal.Decimal `json:"lowest_price,omitempty"`
	Sparkline    []float64        `json:"sparkline,omitempty"`
	StoreMode    models.StoreMode `json:"store_mode,omitempty"`
}

func (i *Items) ListActive(ctx context.Context) ([]ItemSummary, error) {
	rows, err := i.pool.Query(ctx, `
		SELECT
			it.id, it.name, it.url, it.normalized_url, it.store, it.category,
			it.gtin, it.sku, it.brand, it.target_price, it.currency, it.active,
			it.last_fetched_at, it.last_error, it.fail_count, it.created_at,
			COALESCE(st.mode, 'capture_only') AS store_mode,
			(
				SELECT pp.price FROM price_points pp
				WHERE pp.item_id = it.id
				ORDER BY pp.fetched_at DESC LIMIT 1
			) AS current_price,
			(
				SELECT MIN(pp.price) FROM price_points pp WHERE pp.item_id = it.id
			) AS lowest_price
		FROM items it
		LEFT JOIN stores st ON st.domain = it.store
		WHERE it.active = TRUE
		ORDER BY it.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ItemSummary
	for rows.Next() {
		var s ItemSummary
		var cur, low *decimal.Decimal
		if err := rows.Scan(
			&s.ID, &s.Name, &s.URL, &s.NormalizedURL, &s.Store, &s.Category,
			&s.GTIN, &s.SKU, &s.Brand, &s.TargetPrice, &s.Currency, &s.Active,
			&s.LastFetchedAt, &s.LastError, &s.FailCount, &s.CreatedAt,
			&s.StoreMode, &cur, &low,
		); err != nil {
			return nil, err
		}
		s.CurrentPrice = cur
		s.LowestPrice = low
		spark, err := i.sparkline(ctx, s.ID, 30)
		if err != nil {
			return nil, err
		}
		s.Sparkline = spark
		out = append(out, s)
	}
	return out, rows.Err()
}

func (i *Items) sparkline(ctx context.Context, itemID string, limit int) ([]float64, error) {
	rows, err := i.pool.Query(ctx, `
		SELECT price FROM (
			SELECT price, fetched_at FROM price_points
			WHERE item_id = $1
			ORDER BY fetched_at DESC
			LIMIT $2
		) t ORDER BY fetched_at ASC
	`, itemID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []float64
	for rows.Next() {
		var p decimal.Decimal
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		f, _ := p.Float64()
		out = append(out, f)
	}
	return out, rows.Err()
}

func (i *Items) Get(ctx context.Context, id string) (*models.Item, error) {
	row := i.pool.QueryRow(ctx, `
		SELECT id, name, url, normalized_url, store, category, gtin, sku, brand,
		       target_price, currency, active, last_fetched_at, last_error, fail_count, created_at
		FROM items WHERE id = $1
	`, id)
	return scanItem(row)
}

func (i *Items) GetByNormalizedURL(ctx context.Context, normalized string) (*models.Item, error) {
	row := i.pool.QueryRow(ctx, `
		SELECT id, name, url, normalized_url, store, category, gtin, sku, brand,
		       target_price, currency, active, last_fetched_at, last_error, fail_count, created_at
		FROM items WHERE normalized_url = $1 AND active = TRUE
	`, normalized)
	return scanItem(row)
}

type CreateItemParams struct {
	Name          string
	URL           string
	NormalizedURL string
	Store         string
	Category      string
	GTIN          *string
	SKU           *string
	Brand         *string
	TargetPrice   decimal.Decimal
	Currency      string
}

func (i *Items) Create(ctx context.Context, p CreateItemParams) (*models.Item, error) {
	row := i.pool.QueryRow(ctx, `
		INSERT INTO items (
			name, url, normalized_url, store, category, gtin, sku, brand,
			target_price, currency
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, name, url, normalized_url, store, category, gtin, sku, brand,
		          target_price, currency, active, last_fetched_at, last_error, fail_count, created_at
	`, p.Name, p.URL, p.NormalizedURL, p.Store, p.Category, p.GTIN, p.SKU, p.Brand, p.TargetPrice, p.Currency)
	return scanItem(row)
}

type UpdateItemParams struct {
	Name        *string
	Category    *string
	TargetPrice *decimal.Decimal
	Active      *bool
}

func (i *Items) Update(ctx context.Context, id string, p UpdateItemParams) (*models.Item, error) {
	item, err := i.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Name != nil {
		item.Name = *p.Name
	}
	if p.Category != nil {
		item.Category = *p.Category
	}
	if p.TargetPrice != nil {
		item.TargetPrice = *p.TargetPrice
	}
	if p.Active != nil {
		item.Active = *p.Active
	}
	_, err = i.pool.Exec(ctx, `
		UPDATE items SET name=$2, category=$3, target_price=$4, active=$5
		WHERE id=$1
	`, id, item.Name, item.Category, item.TargetPrice, item.Active)
	if err != nil {
		return nil, err
	}
	return i.Get(ctx, id)
}

func (i *Items) SoftDelete(ctx context.Context, id string) error {
	tag, err := i.pool.Exec(ctx, `UPDATE items SET active = FALSE WHERE id = $1 AND active = TRUE`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (i *Items) ListForAutoFetch(ctx context.Context) ([]models.Item, error) {
	rows, err := i.pool.Query(ctx, `
		SELECT it.id, it.name, it.url, it.normalized_url, it.store, it.category,
		       it.gtin, it.sku, it.brand, it.target_price, it.currency, it.active,
		       it.last_fetched_at, it.last_error, it.fail_count, it.created_at
		FROM items it
		INNER JOIN stores st ON st.domain = it.store AND st.mode = 'auto'
		WHERE it.active = TRUE
		ORDER BY it.last_fetched_at NULLS FIRST, it.created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Item
	for rows.Next() {
		item, err := scanItemRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (i *Items) MarkFetchOK(ctx context.Context, id string, name, brand string, gtin, sku *string) error {
	_, err := i.pool.Exec(ctx, `
		UPDATE items SET
			last_fetched_at = now(),
			last_error = NULL,
			fail_count = 0,
			name = CASE WHEN $2 <> '' THEN $2 ELSE name END,
			brand = COALESCE($3, brand),
			gtin = COALESCE($4, gtin),
			sku = COALESCE($5, sku)
		WHERE id = $1
	`, id, name, nullStr(brand), gtin, sku)
	return err
}

func (i *Items) MarkFetchError(ctx context.Context, id, code string) error {
	_, err := i.pool.Exec(ctx, `
		UPDATE items SET
			last_fetched_at = now(),
			last_error = $2,
			fail_count = fail_count + 1
		WHERE id = $1
	`, id, code)
	return err
}

func (i *Items) LatestPrice(ctx context.Context, itemID string) (*models.PricePoint, error) {
	row := i.pool.QueryRow(ctx, `
		SELECT id, item_id, price, currency, in_stock, source, fetched_at
		FROM price_points WHERE item_id = $1
		ORDER BY fetched_at DESC LIMIT 1
	`, itemID)
	return scanPrice(row)
}

func (i *Items) LowestPrice(ctx context.Context, itemID string) (*decimal.Decimal, error) {
	var p *decimal.Decimal
	err := i.pool.QueryRow(ctx, `SELECT MIN(price) FROM price_points WHERE item_id = $1`, itemID).Scan(&p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (i *Items) ListPrices(ctx context.Context, itemID string) ([]models.PricePoint, error) {
	rows, err := i.pool.Query(ctx, `
		SELECT id, item_id, price, currency, in_stock, source, fetched_at
		FROM price_points WHERE item_id = $1
		ORDER BY fetched_at ASC
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.PricePoint
	for rows.Next() {
		var pp models.PricePoint
		if err := rows.Scan(&pp.ID, &pp.ItemID, &pp.Price, &pp.Currency, &pp.InStock, &pp.Source, &pp.FetchedAt); err != nil {
			return nil, err
		}
		out = append(out, pp)
	}
	return out, rows.Err()
}

func (i *Items) InsertPrice(ctx context.Context, itemID string, price decimal.Decimal, currency string, inStock *bool, source models.PriceSource) (*models.PricePoint, error) {
	row := i.pool.QueryRow(ctx, `
		INSERT INTO price_points (item_id, price, currency, in_stock, source)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, item_id, price, currency, in_stock, source, fetched_at
	`, itemID, price, currency, inStock, source)
	return scanPrice(row)
}

func (i *Items) InsertNotification(ctx context.Context, itemID string, typ models.NotificationType, price decimal.Decimal) error {
	_, err := i.pool.Exec(ctx, `
		INSERT INTO notifications (item_id, type, price) VALUES ($1,$2,$3)
	`, itemID, typ, price)
	return err
}

func (i *Items) ListNotifications(ctx context.Context, itemID string) ([]models.Notification, error) {
	rows, err := i.pool.Query(ctx, `
		SELECT id, item_id, type, price, sent_at FROM notifications
		WHERE item_id = $1 ORDER BY sent_at DESC
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Notification
	for rows.Next() {
		var n models.Notification
		if err := rows.Scan(&n.ID, &n.ItemID, &n.Type, &n.Price, &n.SentAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (i *Items) SavePushSubscription(ctx context.Context, endpoint, p256dh, auth string) error {
	_, err := i.pool.Exec(ctx, `
		INSERT INTO push_subscriptions (endpoint, p256dh, auth)
		VALUES ($1,$2,$3)
		ON CONFLICT (endpoint) DO UPDATE SET p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth
	`, endpoint, p256dh, auth)
	return err
}

func (i *Items) DeletePushSubscription(ctx context.Context, endpoint string) error {
	_, err := i.pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	return err
}

func (i *Items) ListPushSubscriptions(ctx context.Context) ([]models.PushSubscription, error) {
	rows, err := i.pool.Query(ctx, `SELECT id, endpoint, p256dh, auth, created_at FROM push_subscriptions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.PushSubscription
	for rows.Next() {
		var s models.PushSubscription
		if err := rows.Scan(&s.ID, &s.Endpoint, &s.P256dh, &s.Auth, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanItem(row scannable) (*models.Item, error) {
	var it models.Item
	err := row.Scan(
		&it.ID, &it.Name, &it.URL, &it.NormalizedURL, &it.Store, &it.Category,
		&it.GTIN, &it.SKU, &it.Brand, &it.TargetPrice, &it.Currency, &it.Active,
		&it.LastFetchedAt, &it.LastError, &it.FailCount, &it.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("scan item: %w", err)
	}
	return &it, nil
}

func scanItemRows(rows pgx.Rows) (*models.Item, error) {
	return scanItem(rows)
}

func scanPrice(row scannable) (*models.PricePoint, error) {
	var pp models.PricePoint
	err := row.Scan(&pp.ID, &pp.ItemID, &pp.Price, &pp.Currency, &pp.InStock, &pp.Source, &pp.FetchedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &pp, nil
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
