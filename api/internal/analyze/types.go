package analyze

import (
	"fmt"
	"net/url"
	"strings"
)

type Result struct {
	Product        Product         `json:"product"`
	CurrentPrices  []StorePrice    `json:"current_prices"`
	HistoricalLow  *HistoricalLow  `json:"historical_low,omitempty"`
	Predecessor    *Predecessor    `json:"predecessor,omitempty"`
	BestMonth      string          `json:"best_month"`
	BestPeriodNote string          `json:"best_period_note"`
	Confidence     string          `json:"confidence"`
	Reasoning      string          `json:"reasoning"`
	Sources        []Source        `json:"sources"`
}

type Product struct {
	Name    string  `json:"name"`
	Brand   string  `json:"brand"`
	Variant string  `json:"variant"`
	GTIN    *string `json:"gtin,omitempty"`
}

type StorePrice struct {
	Store string  `json:"store"`
	Price float64 `json:"price"`
	URL   string  `json:"url"`
}

type HistoricalLow struct {
	Price  float64 `json:"price"`
	Date   string  `json:"date"`
	Source string  `json:"source"`
}

type Predecessor struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
}

type Source struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Validate checks required fields and basic URL hygiene for the analyze JSON schema.
func Validate(r *Result) error {
	if r == nil {
		return fmt.Errorf("empty result")
	}
	if strings.TrimSpace(r.Product.Name) == "" {
		return fmt.Errorf("product.name required")
	}
	if strings.TrimSpace(r.BestMonth) == "" {
		return fmt.Errorf("best_month required")
	}
	switch strings.ToLower(strings.TrimSpace(r.Confidence)) {
	case "low", "medium", "high":
		r.Confidence = strings.ToLower(strings.TrimSpace(r.Confidence))
	default:
		return fmt.Errorf("confidence must be low|medium|high")
	}
	if strings.TrimSpace(r.Reasoning) == "" {
		return fmt.Errorf("reasoning required")
	}
	if len(r.CurrentPrices) == 0 {
		return fmt.Errorf("current_prices required")
	}
	for i, p := range r.CurrentPrices {
		if strings.TrimSpace(p.Store) == "" {
			return fmt.Errorf("current_prices[%d].store required", i)
		}
		if p.Price <= 0 {
			return fmt.Errorf("current_prices[%d].price must be > 0", i)
		}
		if err := requireHTTPURL(p.URL); err != nil {
			return fmt.Errorf("current_prices[%d].url: %w", i, err)
		}
	}
	if r.HistoricalLow != nil {
		if r.HistoricalLow.Price <= 0 {
			return fmt.Errorf("historical_low.price must be > 0")
		}
	}
	for i, s := range r.Sources {
		if strings.TrimSpace(s.Title) == "" {
			return fmt.Errorf("sources[%d].title required", i)
		}
		if err := requireHTTPURL(s.URL); err != nil {
			return fmt.Errorf("sources[%d].url: %w", i, err)
		}
	}
	if r.Product.GTIN != nil {
		g := strings.TrimSpace(*r.Product.GTIN)
		if g == "" {
			r.Product.GTIN = nil
		} else {
			r.Product.GTIN = &g
		}
	}
	return nil
}

func requireHTTPURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be http(s)")
	}
	if u.Host == "" {
		return fmt.Errorf("missing host")
	}
	return nil
}

// NormalizeQueryKey collapses whitespace and lowercases for cache lookup.
func NormalizeQueryKey(q string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(q))), " ")
}
