package ingest

import (
	"net/url"
	"strings"
)

// SanitizeURL keeps http, https, and overcast URLs and drops every other
// scheme, including javascript, data, and vbscript.
func SanitizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !cleanURL(raw) {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		if parsed.Hostname() == "" {
			return ""
		}
		return raw
	case "overcast":
		return raw
	default:
		return ""
	}
}

func cleanURL(raw string) bool {
	for i := 0; i < len(raw); i++ {
		if raw[i] < 0x20 || raw[i] == 0x7f {
			return false
		}
	}
	return true
}
