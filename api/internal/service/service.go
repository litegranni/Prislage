package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"prislage/internal/fetch"
	"prislage/internal/models"
	"prislage/internal/notify"
	"prislage/internal/store"
	"prislage/internal/urlx"
)

type Service struct {
	Items   *store.Items
	Stores  *store.Stores
	Fetcher fetch.Fetcher
	Push    PushSender
	Log     *slog.Logger
}

type PushSender interface {
	Send(ctx context.Context, title, body string) error
}

type noopPush struct{}

func (noopPush) Send(context.Context, string, string) error { return nil }

func New(items *store.Items, stores *store.Stores, fetcher fetch.Fetcher, push PushSender, log *slog.Logger) *Service {
	if push == nil {
		push = noopPush{}
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{Items: items, Stores: stores, Fetcher: fetcher, Push: push, Log: log}
}

type CreateFromURLInput struct {
	URL         string
	TargetPrice decimal.Decimal
	Category    string
}

func (s *Service) CreateFromURL(ctx context.Context, in CreateFromURLInput) (*store.ItemSummary, error) {
	norm, err := urlx.Normalize(in.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if existing, err := s.Items.GetByNormalizedURL(ctx, norm); err == nil {
		return s.summaryOf(ctx, existing.ID)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	mode, domain, err := s.Stores.ModeForURL(ctx, norm)
	if err != nil {
		return nil, err
	}
	if mode == models.StoreModeCaptureOnly {
		return nil, fmt.Errorf("%w: %s är capture_only — använd bookmarklet / POST /api/capture", errCaptureOnly, domain)
	}

	res, err := s.Fetcher.Fetch(ctx, norm)
	if err != nil {
		return nil, err
	}

	item, err := s.Items.Create(ctx, store.CreateItemParams{
		Name:          nonEmpty(res.Name, domain),
		URL:           in.URL,
		NormalizedURL: norm,
		Store:         domain,
		Category:      in.Category,
		GTIN:          strPtr(res.GTIN),
		SKU:           strPtr(res.SKU),
		Brand:         strPtr(res.Brand),
		TargetPrice:   in.TargetPrice,
		Currency:      defaultCurrency(res.Currency),
	})
	if err != nil {
		return nil, err
	}

	if _, err := s.recordPrice(ctx, item, res.Price, res.Currency, res.InStock, res.Source); err != nil {
		return nil, err
	}
	_ = s.Items.MarkFetchOK(ctx, item.ID, res.Name, res.Brand, strPtr(res.GTIN), strPtr(res.SKU))
	return s.summaryOf(ctx, item.ID)
}

var errCaptureOnly = errors.New("capture_only store")

func IsCaptureOnly(err error) bool {
	return errors.Is(err, errCaptureOnly)
}

type CaptureInput struct {
	URL         string
	HTML        string
	JSONLD      string
	TargetPrice *decimal.Decimal
}

func (s *Service) Capture(ctx context.Context, in CaptureInput) (*store.ItemSummary, error) {
	norm, err := urlx.Normalize(in.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	domain := urlx.StoreHost(norm)
	_, _, _ = s.Stores.ModeForURL(ctx, norm) // ensure store row exists

	var res *fetch.Result
	switch {
	case strings.TrimSpace(in.HTML) != "":
		res, err = fetch.ParseHTML([]byte(in.HTML), norm)
	case strings.TrimSpace(in.JSONLD) != "":
		wrapped := `<html><head><script type="application/ld+json">` + in.JSONLD + `</script></head><body></body></html>`
		res, err = fetch.ParseHTML([]byte(wrapped), norm)
	default:
		return nil, fmt.Errorf("html or jsonld required")
	}
	if err != nil {
		return nil, err
	}

	item, err := s.Items.GetByNormalizedURL(ctx, norm)
	if errors.Is(err, pgx.ErrNoRows) {
		target := res.Price
		if in.TargetPrice != nil {
			target = *in.TargetPrice
		}
		item, err = s.Items.Create(ctx, store.CreateItemParams{
			Name:          nonEmpty(res.Name, domain),
			URL:           in.URL,
			NormalizedURL: norm,
			Store:         domain,
			GTIN:          strPtr(res.GTIN),
			SKU:           strPtr(res.SKU),
			Brand:         strPtr(res.Brand),
			TargetPrice:   target,
			Currency:      defaultCurrency(res.Currency),
		})
	}
	if err != nil {
		return nil, err
	}

	if _, err := s.recordPrice(ctx, item, res.Price, res.Currency, res.InStock, models.SourceCapture); err != nil {
		return nil, err
	}
	_ = s.Items.MarkFetchOK(ctx, item.ID, res.Name, res.Brand, strPtr(res.GTIN), strPtr(res.SKU))
	return s.summaryOf(ctx, item.ID)
}

func (s *Service) FetchItem(ctx context.Context, item *models.Item) error {
	res, err := s.Fetcher.Fetch(ctx, item.URL)
	if err != nil {
		code := fetch.CodeOf(err)
		if code == "" {
			code = fetch.ErrCodeHTTPError
		}
		_ = s.Items.MarkFetchError(ctx, item.ID, code)
		return err
	}
	fresh, err := s.Items.Get(ctx, item.ID)
	if err != nil {
		return err
	}
	if _, err := s.recordPrice(ctx, fresh, res.Price, res.Currency, res.InStock, res.Source); err != nil {
		_ = s.Items.MarkFetchError(ctx, item.ID, fetch.ErrCodeParseError)
		return err
	}
	return s.Items.MarkFetchOK(ctx, item.ID, res.Name, res.Brand, strPtr(res.GTIN), strPtr(res.SKU))
}

func (s *Service) FetchAllAuto(ctx context.Context, ratePerMinute int) (ok, fail int) {
	items, err := s.Items.ListForAutoFetch(ctx)
	if err != nil {
		s.Log.Error("list auto items", "err", err)
		return 0, 0
	}
	if ratePerMinute <= 0 {
		ratePerMinute = 10
	}
	delay := time.Minute / time.Duration(ratePerMinute)
	for i, item := range items {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ok, fail
			case <-time.After(delay):
			}
		}
		s.Log.Info("fetch item", "id", item.ID, "store", item.Store, "url", item.URL)
		if err := s.FetchItem(ctx, &item); err != nil {
			s.Log.Error("fetch failed", "id", item.ID, "err", err)
			fail++
			continue
		}
		ok++
	}
	return ok, fail
}

func (s *Service) recordPrice(ctx context.Context, item *models.Item, price decimal.Decimal, currency string, inStock *bool, source models.PriceSource) (*models.PricePoint, error) {
	var prevPrice *decimal.Decimal
	if latest, err := s.Items.LatestPrice(ctx, item.ID); err == nil {
		p := latest.Price
		prevPrice = &p
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	prevLow, err := s.Items.LowestPrice(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	pp, err := s.Items.InsertPrice(ctx, item.ID, price, defaultCurrency(currency), inStock, source)
	if err != nil {
		return nil, err
	}

	for _, typ := range notify.Decide(prevPrice, prevLow, price, item.TargetPrice) {
		if err := s.Items.InsertNotification(ctx, item.ID, typ, price); err != nil {
			s.Log.Error("insert notification", "err", err)
			continue
		}
		title := "Prisläge"
		body := fmt.Sprintf("%s: %s %s (%s)", item.Name, price.StringFixed(2), defaultCurrency(currency), typ)
		if err := s.Push.Send(ctx, title, body); err != nil {
			s.Log.Error("push send", "err", err)
		}
	}
	return pp, nil
}

func (s *Service) summaryOf(ctx context.Context, id string) (*store.ItemSummary, error) {
	list, err := s.Items.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	item, err := s.Items.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &store.ItemSummary{Item: *item}, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func defaultCurrency(c string) string {
	c = strings.ToUpper(strings.TrimSpace(c))
	if c == "" {
		return "SEK"
	}
	return c
}
