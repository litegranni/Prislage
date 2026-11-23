package handlers

import "testing"

func TestParseCORSAllowlist(t *testing.T) {
	if got := parseCORSAllowlist("*"); got != nil {
		t.Fatalf("star => %#v", got)
	}
	if got := parseCORSAllowlist(""); got != nil {
		t.Fatalf("empty => %#v", got)
	}
	got := parseCORSAllowlist("http://localhost:4188, https://prislage.example.ts.net/")
	if len(got) != 2 || got[0] != "http://localhost:4188" || got[1] != "https://prislage.example.ts.net" {
		t.Fatalf("got %#v", got)
	}
}

func TestOriginAllowed(t *testing.T) {
	allow := []string{"http://localhost:4188", "https://app.example"}
	if !originAllowed("http://localhost:4188", allow) {
		t.Fatal("expected allow localhost")
	}
	if originAllowed("https://www.elgiganten.se", allow) {
		t.Fatal("store origin must be denied")
	}
	if originAllowed("http://localhost:4188", nil) {
		t.Fatal("empty allowlist denies all cross-origin")
	}
}
