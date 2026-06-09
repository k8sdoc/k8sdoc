// Package util provides shared helper functions for k8sdoc.
package util

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
)


func GetCacheKey(provider, language, text string) string {
	h := sha256.New()
	h.Write([]byte(provider + language + text))
	return fmt.Sprintf("%x", h.Sum(nil))
}


func MaskString(s string) string {
	if len(s) <= 2 {
		return strings.Repeat("*", len(s))
	}
	return string(s[0]) + strings.Repeat("*", len(s)-2) + string(s[len(s)-1])
}

func NewHeaders(headerStrings []string) []http.Header {
	var headers []http.Header
	for _, h := range headerStrings {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) != 2 {
			continue
		}
		hdr := http.Header{}
		hdr.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		headers = append(headers, hdr)
	}
	return headers
}

func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
