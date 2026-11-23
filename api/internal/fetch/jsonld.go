package fetch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/shopspring/decimal"

	"prislage/internal/gtin"
	"prislage/internal/models"
)

type jsonLDObject map[string]json.RawMessage

func parseJSONLD(doc *goquery.Document) (*Result, error) {
	var lastErr error
	var best *Result

	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return
		}
		nodes, err := decodeJSONLD(raw)
		if err != nil {
			lastErr = err
			return
		}
		for _, node := range nodes {
			if res, ok := resultFromProductNode(node); ok {
				best = res
				return
			}
		}
	})

	if best != nil {
		return best, nil
	}
	if lastErr != nil {
		return nil, fmt.Errorf("json-ld: %w", lastErr)
	}
	return nil, fmt.Errorf("json-ld: no Product/Offer found")
}

func decodeJSONLD(raw string) ([]jsonLDObject, error) {
	raw = strings.TrimSpace(raw)
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.UseNumber()

	var top any
	if err := dec.Decode(&top); err != nil {
		return nil, err
	}

	var out []jsonLDObject
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			obj := toRawObject(t)
			out = append(out, obj)
			if graph, ok := t["@graph"]; ok {
				walk(graph)
			}
			// Nested product under ItemList / etc.
			for _, key := range []string{"item", "itemListElement", "mainEntity", "hasVariant", "offers"} {
				if nested, ok := t[key]; ok {
					walk(nested)
				}
			}
		case []any:
			for _, el := range t {
				walk(el)
			}
		}
	}
	walk(top)
	return out, nil
}

func toRawObject(m map[string]any) jsonLDObject {
	obj := make(jsonLDObject, len(m))
	for k, v := range m {
		b, err := json.Marshal(v)
		if err != nil {
			continue
		}
		obj[k] = b
	}
	return obj
}

func resultFromProductNode(node jsonLDObject) (*Result, bool) {
	if !typeIncludes(node, "Product", "ProductGroup", "IndividualProduct") {
		// Some pages put Offer at top level with an itemOffered Product.
		if typeIncludes(node, "Offer", "AggregateOffer") {
			return resultFromOfferNode(node, "")
		}
		return nil, false
	}

	name := stringField(node, "name")
	brand := brandField(node)
	gtinVal, skuVal := classifyProductIDs(node)

	offersRaw, ok := node["offers"]
	if !ok {
		return nil, false
	}
	offerNodes := expandOffers(offersRaw)
	for _, offer := range offerNodes {
		if res, ok := resultFromOfferNode(offer, name); ok {
			if res.Brand == "" {
				res.Brand = brand
			}
			if res.GTIN == "" {
				res.GTIN = gtinVal
			}
			if res.SKU == "" {
				res.SKU = skuVal
			}
			if res.Name == "" {
				res.Name = name
			}
			res.Source = models.SourceJSONLD
			return res, true
		}
	}
	return nil, false
}

// classifyProductIDs prefers real GTINs; non-GTIN identifiers become SKU.
func classifyProductIDs(node jsonLDObject) (gtinVal, skuVal string) {
	for _, key := range []string{"gtin13", "gtin14", "gtin12", "gtin8", "gtin"} {
		if raw := stringField(node, key); raw != "" {
			g, s := gtin.Classify(raw)
			if gtinVal == "" && g != "" {
				gtinVal = g
			} else if skuVal == "" && s != "" {
				skuVal = s
			}
		}
	}
	for _, key := range []string{"sku", "productID", "mpn"} {
		if raw := stringField(node, key); raw != "" {
			g, s := gtin.Classify(raw)
			if gtinVal == "" && g != "" {
				gtinVal = g
				continue
			}
			if skuVal == "" && s != "" {
				skuVal = s
			}
		}
	}
	return gtinVal, skuVal
}

func resultFromOfferNode(node jsonLDObject, productName string) (*Result, bool) {
	priceRaw := firstStringField(node, "price", "lowPrice")
	if priceRaw == "" {
		// priceSpecification
		if spec, ok := node["priceSpecification"]; ok {
			for _, s := range expandOffers(spec) {
				priceRaw = firstStringField(s, "price")
				if priceRaw != "" {
					if cur := stringField(s, "priceCurrency"); cur != "" {
						node["priceCurrency"] = mustRaw(cur)
					}
					break
				}
			}
		}
	}
	if priceRaw == "" {
		return nil, false
	}

	price, err := parsePrice(priceRaw)
	if err != nil {
		return nil, false
	}

	currency := strings.ToUpper(stringField(node, "priceCurrency"))
	avail := ParseAvailability(stringField(node, "availability"))
	name := productName
	if name == "" {
		name = stringField(node, "name")
	}

	return &Result{
		Name:     name,
		Price:    price,
		Currency: currency,
		InStock:  avail,
		Source:   models.SourceJSONLD,
	}, true
}

func expandOffers(raw json.RawMessage) []jsonLDObject {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		out := make([]jsonLDObject, 0, len(arr))
		for _, el := range arr {
			var obj jsonLDObject
			if json.Unmarshal(el, &obj) == nil {
				out = append(out, obj)
			}
		}
		return out
	}
	var obj jsonLDObject
	if json.Unmarshal(raw, &obj) == nil {
		return []jsonLDObject{obj}
	}
	return nil
}

func typeIncludes(node jsonLDObject, types ...string) bool {
	raw, ok := node["@type"]
	if !ok {
		return false
	}
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return matchType(single, types...)
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		for _, t := range many {
			if matchType(t, types...) {
				return true
			}
		}
	}
	return false
}

func matchType(got string, want ...string) bool {
	got = strings.TrimPrefix(got, "https://schema.org/")
	got = strings.TrimPrefix(got, "http://schema.org/")
	for _, w := range want {
		if strings.EqualFold(got, w) {
			return true
		}
	}
	return false
}

func stringField(node jsonLDObject, key string) string {
	raw, ok := node[key]
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	// Language maps or nested name objects
	var obj map[string]any
	if json.Unmarshal(raw, &obj) == nil {
		if v, ok := obj["@value"].(string); ok {
			return strings.TrimSpace(v)
		}
		if v, ok := obj["name"].(string); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstStringField(node jsonLDObject, keys ...string) string {
	for _, k := range keys {
		if v := stringField(node, k); v != "" {
			return v
		}
	}
	return ""
}

func brandField(node jsonLDObject) string {
	raw, ok := node["brand"]
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var obj jsonLDObject
	if json.Unmarshal(raw, &obj) == nil {
		return stringField(obj, "name")
	}
	return ""
}

func mustRaw(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

func parsePrice(raw string) (decimal.Decimal, error) {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.TrimSpace(s)

	// Strip currency symbols/letters around the number.
	replacer := strings.NewReplacer(
		"kr", "", "KR", "", "SEK", "", "sek", "",
		"€", "", "EUR", "", "$", "", "USD", "",
		"£", "", "GBP", "", "NOK", "", "DKK", "",
		" ", "",
	)
	s = replacer.Replace(s)

	// Swedish/EU format: 1.299,00 or 1299,00
	if strings.Contains(s, ",") && strings.Contains(s, ".") {
		// Assume . thousands, , decimal
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	} else if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ",", ".")
	}

	s = strings.Trim(s, ".-")
	if s == "" {
		return decimal.Zero, fmt.Errorf("empty price")
	}
	return decimal.NewFromString(s)
}
