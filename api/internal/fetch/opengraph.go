package fetch

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"prislage/internal/models"
)

func parseOpenGraph(doc *goquery.Document) (*Result, error) {
	meta := func(prop string) string {
		// property= (OG) or name= (Twitter / generic)
		var val string
		doc.Find("meta").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			p, _ := s.Attr("property")
			n, _ := s.Attr("name")
			if strings.EqualFold(p, prop) || strings.EqualFold(n, prop) {
				val, _ = s.Attr("content")
				return false
			}
			return true
		})
		return strings.TrimSpace(val)
	}

	priceRaw := firstNonEmpty(
		meta("product:price:amount"),
		meta("og:price:amount"),
		meta("twitter:data1"),
		meta("price"),
	)
	if priceRaw == "" {
		return nil, fmt.Errorf("opengraph: no price meta tag")
	}

	// twitter:data1 sometimes looks like "1 299 kr"
	price, err := parsePrice(priceRaw)
	if err != nil {
		return nil, fmt.Errorf("opengraph: parse price %q: %w", priceRaw, err)
	}

	currency := strings.ToUpper(firstNonEmpty(
		meta("product:price:currency"),
		meta("og:price:currency"),
	))

	name := firstNonEmpty(
		meta("og:title"),
		meta("twitter:title"),
		doc.Find("title").First().Text(),
	)
	name = strings.TrimSpace(name)

	avail := ParseAvailability(meta("product:availability"))
	brand := meta("product:brand")

	return &Result{
		Name:     name,
		Price:    price,
		Currency: currency,
		InStock:  avail,
		Brand:    brand,
		Source:   models.SourceOG,
	}, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
