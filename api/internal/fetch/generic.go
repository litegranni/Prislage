package fetch

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"prislage/internal/urlx"
)

// GenericFetcher fetches any store page and extracts Product data via JSON-LD,
// falling back to Open Graph / meta tags.
type GenericFetcher struct {
	HTTP *HTTPClient
}

func NewGenericFetcher(httpClient *HTTPClient) *GenericFetcher {
	if httpClient == nil {
		httpClient = NewHTTPClient("")
	}
	return &GenericFetcher{HTTP: httpClient}
}

func (g *GenericFetcher) Match(_ string) bool { return true }

func (g *GenericFetcher) Fetch(ctx context.Context, rawURL string) (*Result, error) {
	body, finalURL, err := g.HTTP.Get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	res, err := ParseHTML(body, finalURL)
	if err != nil {
		return nil, err
	}
	if res.Store == "" {
		res.Store = urlx.StoreHost(finalURL)
	}
	return res, nil
}

// ParseHTML extracts product price/metadata from an HTML document.
// Prefer schema.org JSON-LD; fall back to Open Graph / meta tags.
func ParseHTML(body []byte, pageURL string) (*Result, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	if res, err := parseJSONLD(doc); err == nil {
		finalizeResult(res, pageURL)
		return res, nil
	} else {
		jsonldErr := err
		if res, err := parseOpenGraph(doc); err == nil {
			finalizeResult(res, pageURL)
			return res, nil
		} else {
			return nil, fmt.Errorf("no price found (json-ld: %v; og: %v)", jsonldErr, err)
		}
	}
}

func finalizeResult(res *Result, pageURL string) {
	if res.Currency == "" {
		res.Currency = CurrencyFromURL(pageURL)
	}
	res.Currency = strings.ToUpper(res.Currency)
	if res.Store == "" {
		res.Store = urlx.StoreHost(pageURL)
	}
	res.Name = strings.TrimSpace(res.Name)
	res.Brand = strings.TrimSpace(res.Brand)
	res.GTIN = strings.TrimSpace(res.GTIN)
}
