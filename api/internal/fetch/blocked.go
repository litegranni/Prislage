package fetch

import (
	"net/url"
	"strings"

	"prislage/internal/urlx"
)

// knownBlockedHosts are stores we skip entirely (bot walls we will not bypass).
// Matched against the registrable-ish host from urlx.StoreHost (www. stripped).
var knownBlockedHosts = map[string]string{
	"elgiganten.se": "Elgiganten blockerar automatiserad hämtning (Vercel Security Checkpoint)",
}

// IsKnownBlockedStore reports whether the URL host is on the hard blocklist.
// POST /api/items should reject these before attempting a fetch.
func IsKnownBlockedStore(rawURL string) (blocked bool, reason string) {
	host := urlx.StoreHost(rawURL)
	if host == "" {
		if u, err := url.Parse(strings.TrimSpace(rawURL)); err == nil {
			host = strings.TrimPrefix(strings.ToLower(u.Host), "www.")
		}
	}
	if reason, ok := knownBlockedHosts[host]; ok {
		return true, reason
	}
	// Also match subdomains like shop.elgiganten.se
	for blockedHost, reason := range knownBlockedHosts {
		if host == blockedHost || strings.HasSuffix(host, "."+blockedHost) {
			return true, reason
		}
	}
	return false, ""
}

// checkpointSignatures indicate bot-challenge / WAF interstitial pages.
var checkpointSignatures = []string{
	"vercel security checkpoint",
	"verifying your browser",
	"enable javascript to continue",
	"cf-browser-verification",
	"cf-challenge",
	"just a moment...",
	"attention required! | cloudflare",
	"cdn-cgi/challenge-platform",
	"checking your browser before accessing",
	"access denied",
	"_pxcaptcha",
	"perimeterx",
	"datadome",
}

// LooksLikeCheckpoint reports whether HTML looks like a bot/WAF challenge page
// rather than a product document.
func LooksLikeCheckpoint(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	lower := strings.ToLower(string(body))
	for _, sig := range checkpointSignatures {
		if strings.Contains(lower, sig) {
			return true
		}
	}
	return false
}

func classifyHTTPStatus(status int, body []byte) *Error {
	if status == httpStatusForbidden || status == httpStatusTooManyRequests {
		msg := "store blocked the request"
		if LooksLikeCheckpoint(body) {
			msg = "bot checkpoint / WAF challenge page"
		}
		return &Error{Code: ErrCodeBlocked, Message: msg, StatusCode: status}
	}
	if LooksLikeCheckpoint(body) {
		return &Error{
			Code:       ErrCodeBlocked,
			Message:    "bot checkpoint / WAF challenge page",
			StatusCode: status,
		}
	}
	if status < 200 || status >= 300 {
		return &Error{
			Code:       ErrCodeHTTPError,
			Message:    "unexpected HTTP status",
			StatusCode: status,
		}
	}
	return nil
}

const (
	httpStatusForbidden       = 403
	httpStatusTooManyRequests = 429
)
