package fetch

import (
	"errors"
	"fmt"
)

// Codes written to items.last_error when a fetch fails.
const (
	ErrCodeBlocked      = "blocked"
	ErrCodeNoPriceFound = "no_price_found"
	ErrCodeParseError   = "parse_error"
	ErrCodeHTTPError    = "http_error"
)

// Error is a classified fetch/parse failure suitable for items.last_error.
type Error struct {
	Code       string
	Message    string
	StatusCode int
	Store      string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s: %s (HTTP %d)", e.Code, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Is(target error) bool {
	var other *Error
	if !errors.As(target, &other) {
		return false
	}
	return e.Code == other.Code
}

func NewError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func CodeOf(err error) string {
	var fe *Error
	if errors.As(err, &fe) {
		return fe.Code
	}
	return ""
}
