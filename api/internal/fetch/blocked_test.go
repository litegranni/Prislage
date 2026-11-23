package fetch

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseHTML_CheckpointIsBlocked(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "live", "elgiganten-323263.html")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read checkpoint fixture: %v", err)
	}

	_, err = ParseHTML(body, "https://www.elgiganten.se/product/x/323263")
	if err == nil {
		t.Fatal("expected blocked error")
	}
	if CodeOf(err) != ErrCodeBlocked {
		t.Fatalf("code = %q, want %q (err=%v)", CodeOf(err), ErrCodeBlocked, err)
	}
	if !LooksLikeCheckpoint(body) {
		t.Fatal("LooksLikeCheckpoint should be true for fixture")
	}
}

func TestClassifyHTTPStatus(t *testing.T) {
	if fe := classifyHTTPStatus(429, []byte("rate limit")); fe == nil || fe.Code != ErrCodeBlocked {
		t.Fatalf("429 => %+v", fe)
	}
	if fe := classifyHTTPStatus(403, []byte("no")); fe == nil || fe.Code != ErrCodeBlocked {
		t.Fatalf("403 => %+v", fe)
	}
	if fe := classifyHTTPStatus(500, []byte("oops")); fe == nil || fe.Code != ErrCodeHTTPError {
		t.Fatalf("500 => %+v", fe)
	}
	if fe := classifyHTTPStatus(200, []byte(`<title>Vercel Security Checkpoint</title>`)); fe == nil || fe.Code != ErrCodeBlocked {
		t.Fatalf("200 checkpoint => %+v", fe)
	}
	if fe := classifyHTTPStatus(200, []byte(`<html><body>ok</body></html>`)); fe != nil {
		t.Fatalf("200 ok => %+v", fe)
	}
}

func TestParseHTML_NoPriceCode(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "product_no_price.html")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParseHTML(body, "https://shop.example/empty")
	if CodeOf(err) != ErrCodeNoPriceFound {
		t.Fatalf("code = %q, err=%v", CodeOf(err), err)
	}
}
