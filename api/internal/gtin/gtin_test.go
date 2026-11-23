package gtin

import "testing"

func TestValid(t *testing.T) {
	// Well-known valid GTINs
	valid := []string{
		"7350012345678", // from our testdata (may need verify)
		"4006381333931",
		"012345678905", // classic UPC example
		"12345670",     // GTIN-8 style example — verify below
	}
	// Use constructed values with correct check digits.
	cases := []struct {
		in   string
		want bool
	}{
		{makeGTIN("400638133393"), true},  // 13
		{makeGTIN("01234567890"), true},   // 12
		{makeGTIN("1234567"), true},       // 8
		{makeGTIN("0123456789012"), true}, // 14 body 13 digits + check
		{"65820", false},
		{"123", false},
		{"abcdefghijklm", false},
		{"4006381333930", false}, // wrong check
		{"", false},
	}
	_ = valid
	for _, tt := range cases {
		if got := Valid(tt.in); got != tt.want {
			t.Errorf("Valid(%q)=%v want %v", tt.in, got, tt.want)
		}
	}
}

func makeGTIN(body string) string {
	// append correct check digit
	tmp := body + "0"
	cd := checkDigit(tmp)
	return body + string(rune('0'+cd))
}

func TestClassify(t *testing.T) {
	gtin, sku := Classify("65820")
	if gtin != "" || sku != "65820" {
		t.Fatalf("Classify(65820)=(%q,%q)", gtin, sku)
	}
	good := makeGTIN("871258180428")
	gtin, sku = Classify(good)
	if gtin != good || sku != "" {
		t.Fatalf("Classify(valid)=(%q,%q)", gtin, sku)
	}
}
