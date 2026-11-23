package gtin

import (
	"strings"
	"unicode"
)

// Valid reports whether s is a GS1 GTIN-8/12/13/14 with a correct check digit.
func Valid(s string) bool {
	digits := digitsOnly(s)
	switch len(digits) {
	case 8, 12, 13, 14:
	default:
		return false
	}
	return checkDigit(digits) == int(digits[len(digits)-1]-'0')
}

// Classify returns (gtin, sku). If raw is a valid GTIN it is returned as gtin;
// otherwise non-empty raw is returned as sku.
func Classify(raw string) (gtin, sku string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	digits := digitsOnly(raw)
	// Prefer pure digit form for GTIN validation when the value is digit-like.
	candidate := raw
	if len(digits) == len(strings.TrimSpace(raw)) {
		candidate = digits
	} else if len(digits) == 8 || len(digits) == 12 || len(digits) == 13 || len(digits) == 14 {
		candidate = digits
	}
	if Valid(candidate) {
		return digitsOnly(candidate), ""
	}
	return "", raw
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// checkDigit computes the GS1 mod-10 check digit for all digits except the last.
func checkDigit(digits string) int {
	body := digits[:len(digits)-1]
	sum := 0
	// From the right (excluding check digit), odd positions weight 3, even weight 1.
	for i := 0; i < len(body); i++ {
		d := int(body[len(body)-1-i] - '0')
		if i%2 == 0 {
			sum += d * 3
		} else {
			sum += d
		}
	}
	return (10 - (sum % 10)) % 10
}
