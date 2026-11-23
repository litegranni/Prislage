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
	if blocked, reason := IsKnownBlockedStore(rawURL); blocked {
		return nil, &Error{
			Code:    ErrCodeBlocked,
			Message: reason,
			Store:   urlx.StoreHost(rawURL),
		}
	}

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
	if LooksLikeCheckpoint(body) {
		return nil, &Error{
			Code:    ErrCodeBlocked,
			Message: "bot checkpoint / WAF challenge page",
			Store:   urlx.StoreHost(pageURL),
		}
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, &Error{
			Code:    ErrCodeParseError,
			Message: fmt.Sprintf("parse html: %v", err),
			Store:   urlx.StoreHost(pageURL),
		}
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
			return nil, &Error{
				Code:    ErrCodeNoPriceFound,
				Message: fmt.Sprintf("json-ld: %v; og: %v", jsonldErr, err),
				Store:   urlx.StoreHost(pageURL),
			}
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
