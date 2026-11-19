package httpclient

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// now is replaceable in tests.
var now = time.Now

// RateLimit holds the rate-limit allowance advertised by a response. Fields
// the server did not send (or sent in an unparseable form) are nil or zero.
type RateLimit struct {
	Limit     *int
	Remaining *int
	Used      *int
	// Reset is when the allowance resets; zero when unknown.
	Reset    time.Time
	Resource string
}

// ResponseInfo describes one HTTP response passed to Options.OnResponse.
// It never contains the response body.
type ResponseInfo struct {
	Operation  string
	Method     string
	Host       string
	StatusCode int
	// Header is a copy of the response headers.
	Header http.Header
	// RetryAfter is the parsed Retry-After header, zero when absent or invalid.
	RetryAfter time.Duration
	RateLimit  RateLimit
}

// parseRetryAfter accepts delta-seconds or an HTTP-date. Invalid or negative
// values yield zero; dates in the past are clamped to zero.
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds <= 0 || seconds > int64(1<<62/time.Second) {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	date, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	if wait := date.Sub(now()); wait > 0 {
		return wait
	}
	return 0
}

// parseRateLimit reads the X-Ratelimit-* headers, falling back to the IETF
// RateLimit-* names. X-Ratelimit-Reset is epoch seconds; RateLimit-Reset is
// delta seconds.
func parseRateLimit(header http.Header) RateLimit {
	var limit RateLimit
	limit.Limit = headerInt(header, "X-Ratelimit-Limit", "Ratelimit-Limit")
	limit.Remaining = headerInt(header, "X-Ratelimit-Remaining", "Ratelimit-Remaining")
	limit.Used = headerInt(header, "X-Ratelimit-Used")
	limit.Resource = strings.TrimSpace(header.Get("X-Ratelimit-Resource"))
	if reset := headerInt(header, "X-Ratelimit-Reset"); reset != nil {
		if *reset > 0 {
			limit.Reset = time.Unix(int64(*reset), 0)
		}
	} else if delta := headerInt(header, "Ratelimit-Reset"); delta != nil && *delta >= 0 {
		limit.Reset = now().Add(time.Duration(*delta) * time.Second)
	}
	return limit
}

func headerInt(header http.Header, names ...string) *int {
	for _, name := range names {
		value := strings.TrimSpace(header.Get(name))
		if value == "" {
			continue
		}
		if parsed, err := strconv.Atoi(value); err == nil {
			return &parsed
		}
	}
	return nil
}
