package fetch

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseHTML_KjellSKUNotGTIN(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "live", "kjell-65820.html")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read kjell fixture: %v", err)
	}

	res, err := ParseHTML(body, "https://www.kjell.com/se/produkter/dator/datorskarmar/portabla-datorskarmar/asus-zenscreen-mb166c-portabel-monitor-156-usb-c-p65820")
	if err != nil {
		t.Fatalf("ParseHTML: %v", err)
	}
	if res.GTIN != "" {
		t.Fatalf("expected empty gtin for Kjell sku-like id, got %q", res.GTIN)
	}
	if res.SKU != "65820" {
		t.Fatalf("sku = %q, want 65820", res.SKU)
	}
	if res.Brand != "Asus" {
		t.Fatalf("brand = %q", res.Brand)
	}
	if res.Price.String() != "1490" {
		t.Fatalf("price = %s", res.Price)
	}
}
