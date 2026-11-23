package urlx

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Normalize cleans a product URL for deduplication:
// - lowercases scheme and host
// - strips fragment
// - drops common tracking query params (utm_*, fbclid, gclid, …)
// - sorts remaining query params for stable comparison
// - trims trailing slash on path (except root)
func Normalize(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty url")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("url must include scheme and host")
	}

	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	u.User = nil

	q := u.Query()
	for key := range q {
		if shouldDropQueryParam(key) {
			q.Del(key)
		}
	}

	keys := make([]string, 0, len(q))
	for key := range q {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var parts []string
	for _, key := range keys {
		for _, value := range q[key] {
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
		}
	}
	if len(parts) > 0 {
		u.RawQuery = strings.Join(parts, "&")
	} else {
		u.RawQuery = ""
	}

	path := u.EscapedPath()
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}
	u.Path = path
	u.RawPath = ""

	return u.String(), nil
}

// StoreHost returns the hostname without leading www.
func StoreHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.ToLower(u.Host)
	return strings.TrimPrefix(host, "www.")
}

func shouldDropQueryParam(key string) bool {
	k := strings.ToLower(key)
	if strings.HasPrefix(k, "utm_") {
		return true
	}
	switch k {
	case "fbclid", "gclid", "gbraid", "wbraid", "msclkid",
		"mc_cid", "yclid", "igshid", "ref", "ref_",
		"aff_id", "affiliate", "cmpid", "campaignid", "adgroupid",
		"creative", "dclid", "srsltid", "si", "spm":
		return true
	default:
		return false
	}
}
