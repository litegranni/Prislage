package fetch

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/shopspring/decimal"

	"prislage/internal/models"
)

func testdata(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read testdata %s: %v", name, err)
	}
	return b
}

func TestParseHTML_JSONLD(t *testing.T) {
	res, err := ParseHTML(testdata(t, "product_jsonld.html"), "https://shop.example/p/kaffe")
	if err != nil {
		t.Fatalf("ParseHTML: %v", err)
	}
	if res.Name != "Kaffemaskin Pro 3000" {
		t.Errorf("name = %q", res.Name)
	}
	if !res.Price.Equal(decimal.RequireFromString("1499.00")) {
		t.Errorf("price = %s", res.Price)
	}
	if res.Currency != "SEK" {
		t.Errorf("currency = %q", res.Currency)
	}
	if res.Brand != "BrewCo" {
		t.Errorf("brand = %q", res.Brand)
	}
	if res.GTIN != "4006381333931" {
		t.Errorf("gtin = %q", res.GTIN)
	}
	if res.SKU != "BRW-3000" {
		t.Errorf("sku = %q", res.SKU)
	}
	if res.Source != models.SourceJSONLD {
		t.Errorf("source = %q", res.Source)
	}
	if res.InStock == nil || !*res.InStock {
		t.Errorf("expected in stock")
	}
	if res.Store != "shop.example" {
		t.Errorf("store = %q", res.Store)
	}
}

func TestParseHTML_JSONLDGraph(t *testing.T) {
	res, err := ParseHTML(testdata(t, "product_jsonld_graph.html"), "https://www.example.se/headphones")
	if err != nil {
		t.Fatalf("ParseHTML: %v", err)
	}
	if res.Name != "Hörlurar Quiet X" {
		t.Errorf("name = %q", res.Name)
	}
	if !res.Price.Equal(decimal.NewFromInt(899)) {
		t.Errorf("price = %s", res.Price)
	}
	if res.Brand != "SoundLab" {
		t.Errorf("brand = %q", res.Brand)
	}
	if res.GTIN != "4006381333931" {
		t.Errorf("gtin = %q", res.GTIN)
	}
	if res.InStock == nil || *res.InStock {
		t.Errorf("expected out of stock")
	}
}

func TestParseHTML_AggregateOffer(t *testing.T) {
	res, err := ParseHTML(testdata(t, "product_aggregate_offer.html"), "https://shop.example/shoes")
	if err != nil {
		t.Fatalf("ParseHTML: %v", err)
	}
	if !res.Price.Equal(decimal.RequireFromString("799.00")) {
		t.Errorf("price = %s (want lowPrice)", res.Price)
	}
	if res.Source != models.SourceJSONLD {
		t.Errorf("source = %q", res.Source)
	}
}

func TestParseHTML_OpenGraph(t *testing.T) {
	res, err := ParseHTML(testdata(t, "product_og.html"), "https://shop.example/magpad")
	if err != nil {
		t.Fatalf("ParseHTML: %v", err)
	}
	if res.Name != "Trådlös laddare MagPad" {
		t.Errorf("name = %q", res.Name)
	}
	if !res.Price.Equal(decimal.NewFromInt(299)) {
		t.Errorf("price = %s", res.Price)
	}
	if res.Currency != "SEK" {
		t.Errorf("currency = %q", res.Currency)
	}
	if res.Brand != "ChargeIt" {
		t.Errorf("brand = %q", res.Brand)
	}
	if res.Source != models.SourceOG {
		t.Errorf("source = %q", res.Source)
	}
	if res.InStock == nil || !*res.InStock {
		t.Errorf("expected in stock")
	}
}

func TestParseHTML_OpenGraphSwedishPrice(t *testing.T) {
	res, err := ParseHTML(testdata(t, "product_og_swedish.html"), "https://butik.se/handduk")
	if err != nil {
		t.Fatalf("ParseHTML: %v", err)
	}
	if !res.Price.Equal(decimal.RequireFromString("1299.00")) {
		t.Errorf("price = %s, want 1299.00", res.Price)
	}
	if res.Source != models.SourceOG {
		t.Errorf("source = %q", res.Source)
	}
}

func TestParseHTML_NoPrice(t *testing.T) {
	_, err := ParseHTML(testdata(t, "product_no_price.html"), "https://shop.example/empty")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParsePrice(t *testing.T) {
	cases := map[string]string{
		"1499.00":  "1499",
		"1.299,00": "1299",
		"1 299 kr": "1299",
		"€12,50":   "12.5",
		"12,5":     "12.5",
	}
	for in, want := range cases {
		got, err := parsePrice(in)
		if err != nil {
			t.Errorf("parsePrice(%q): %v", in, err)
			continue
		}
		if !got.Equal(decimal.RequireFromString(want)) {
			t.Errorf("parsePrice(%q) = %s, want %s", in, got, want)
		}
	}
}
