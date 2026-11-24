package analyze

import "testing"

func TestNormalizeQueryKey(t *testing.T) {
	got := NormalizeQueryKey("  Roborock   Saros 20  ")
	if got != "roborock saros 20" {
		t.Fatalf("got %q", got)
	}
}

func TestValidateOK(t *testing.T) {
	r := &Result{
		Product:       Product{Name: "Roborock Saros 20", Brand: "Roborock", Variant: "Vit"},
		CurrentPrices: []StorePrice{{Store: "inet.se", Price: 9990, URL: "https://www.inet.se/x"}},
		BestMonth:     "november",
		Confidence:    "medium",
		Reasoning:     "Black Friday brukar ge lägsta pris.",
		Sources:       []Source{{Title: "Prisjakt", URL: "https://www.prisjakt.nu/x"}},
	}
	if err := Validate(r); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsJavascriptURL(t *testing.T) {
	r := &Result{
		Product:       Product{Name: "X"},
		CurrentPrices: []StorePrice{{Store: "x", Price: 1, URL: "javascript:alert(1)"}},
		BestMonth:     "januari",
		Confidence:    "low",
		Reasoning:     "test",
	}
	if err := Validate(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseResultJSON(t *testing.T) {
	raw := "```json\n{\"product\":{\"name\":\"A\",\"brand\":\"B\",\"variant\":\"C\"},\"current_prices\":[{\"store\":\"s\",\"price\":10,\"url\":\"https://ex.se/p\"}],\"best_month\":\"maj\",\"best_period_note\":\"n\",\"confidence\":\"high\",\"reasoning\":\"ok\",\"sources\":[]}\n```"
	r, err := parseResultJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(r); err != nil {
		t.Fatal(err)
	}
	if r.Product.Name != "A" || r.CurrentPrices[0].Price != 10 {
		t.Fatalf("%+v", r)
	}
}
