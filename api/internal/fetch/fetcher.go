package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"prislage/internal/models"
	"prislage/internal/urlx"
)

const DefaultUserAgent = "PrislageBot/0.1 (+https://github.com/prislage/prislage; price-watch; contact=prislage@localhost)"

// Result is the product data extracted from a store page.
type Result struct {
	Name     string             `json:"name"`
	Price    decimal.Decimal    `json:"price"`
	Currency string             `json:"currency"`
	InStock  *bool              `json:"in_stock,omitempty"`
	GTIN     string             `json:"gtin,omitempty"`
	Brand    string             `json:"brand,omitempty"`
	Source   models.PriceSource `json:"source"`
	Store    string             `json:"store"`
}

// Fetcher extracts price and product metadata for a URL.
// Store-specific implementations can be registered later.
type Fetcher interface {
	// Match returns true if this fetcher should handle the URL.
	Match(rawURL string) bool
	Fetch(ctx context.Context, rawURL string) (*Result, error)
}

// Registry picks a Fetcher by URL, falling back to Generic.
type Registry struct {
	fetchers []Fetcher
	fallback Fetcher
}

func NewRegistry(fallback Fetcher, storeFetchers ...Fetcher) *Registry {
	return &Registry{fetchers: storeFetchers, fallback: fallback}
}

func (r *Registry) ForURL(rawURL string) Fetcher {
	for _, f := range r.fetchers {
		if f.Match(rawURL) {
			return f
		}
	}
	return r.fallback
}

func (r *Registry) Fetch(ctx context.Context, rawURL string) (*Result, error) {
	return r.ForURL(rawURL).Fetch(ctx, rawURL)
}

// HTTPClient is the shared HTTP client used by fetchers.
type HTTPClient struct {
	Client    *http.Client
	UserAgent string
}

func NewHTTPClient(userAgent string) *HTTPClient {
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	return &HTTPClient{
		Client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		UserAgent: userAgent,
	}
}

func (c *HTTPClient) Get(ctx context.Context, rawURL string) (body []byte, finalURL string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "sv-SE,sv;q=0.9,en;q=0.8")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, 8<<20) // 8 MiB
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", &Error{Code: ErrCodeHTTPError, Message: err.Error()}
	}
	finalURL = resp.Request.URL.String()
	if fe := classifyHTTPStatus(resp.StatusCode, data); fe != nil {
		fe.Store = urlx.StoreHost(finalURL)
		return data, finalURL, fe
	}
	return data, finalURL, nil
}

func ParseAvailability(raw string) *bool {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return nil
	}
	// schema.org URLs or bare terms
	s = strings.TrimPrefix(s, "https://schema.org/")
	s = strings.TrimPrefix(s, "http://schema.org/")
	switch {
	case strings.Contains(s, "instock"), strings.Contains(s, "limitedavailability"),
		strings.Contains(s, "onlineonly"), strings.Contains(s, "preorder"):
		v := true
		return &v
	case strings.Contains(s, "outofstock"), strings.Contains(s, "discontinued"),
		strings.Contains(s, "soldout"):
		v := false
		return &v
	default:
		return nil
	}
}

func CurrencyFromURL(rawURL string) string {
	host := urlx.StoreHost(rawURL)
	switch {
	case strings.HasSuffix(host, ".se"), host == "ikea.com" && strings.Contains(rawURL, "/se/"):
		return "SEK"
	case strings.HasSuffix(host, ".no"):
		return "NOK"
	case strings.HasSuffix(host, ".dk"):
		return "DKK"
	case strings.HasSuffix(host, ".fi"), strings.HasSuffix(host, ".de"), strings.HasSuffix(host, ".fr"):
		return "EUR"
	default:
		return "SEK"
	}
}
