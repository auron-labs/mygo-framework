package httpclient

import (
	"context"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

// Validator is a cached response validator used for conditional requests.
type Validator struct {
	// ETag is sent verbatim as If-None-Match (weak and quoted forms preserved).
	ETag string
	// LastModified is sent verbatim as If-Modified-Since.
	LastModified string
}

// ValidatorFromTime builds a Validator from an ETag and a modification time.
func ValidatorFromTime(etag string, modified time.Time) Validator {
	v := Validator{ETag: etag}
	if !modified.IsZero() {
		v.LastModified = modified.UTC().Format(http.TimeFormat)
	}
	return v
}

// IsZero reports whether the validator carries nothing to send.
func (v Validator) IsZero() bool {
	return v.ETag == "" && v.LastModified == ""
}

// SetValidator adds If-None-Match / If-Modified-Since to the request. An empty
// validator leaves the request unchanged.
func SetValidator(request *resty.Request, v Validator) *resty.Request {
	if v.ETag != "" {
		request.SetHeader("If-None-Match", v.ETag)
	}
	if v.LastModified != "" {
		request.SetHeader("If-Modified-Since", v.LastModified)
	}
	return request
}

// ConditionalRequest is Request with the validator applied.
//
// A 304 Not Modified response is not an error: the call returns a nil error and
// NotModified reports true. Callers keep using their cached body.
func (c *Client) ConditionalRequest(ctx context.Context, operation string, v Validator) *resty.Request {
	return SetValidator(c.Request(ctx, operation), v)
}

// NotModified reports whether the response is a 304 Not Modified.
func NotModified(response *resty.Response) bool {
	return response != nil && response.StatusCode() == http.StatusNotModified
}

// ValidatorFrom reads the validators a server sent so callers can cache them.
func ValidatorFrom(response *resty.Response) Validator {
	if response == nil {
		return Validator{}
	}
	return Validator{
		ETag:         response.Header().Get("ETag"),
		LastModified: response.Header().Get("Last-Modified"),
	}
}
