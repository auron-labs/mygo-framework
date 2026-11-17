package httpclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestSetValidator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		validator     Validator
		wantETag      string
		wantModified  string
		wantNoHeaders bool
	}{
		{name: "both", validator: Validator{ETag: `W/"abc"`, LastModified: "Mon, 02 Jan 2006 15:04:05 GMT"}, wantETag: `W/"abc"`, wantModified: "Mon, 02 Jan 2006 15:04:05 GMT"},
		{name: "etag only", validator: Validator{ETag: `"x"`}, wantETag: `"x"`},
		{name: "empty", validator: Validator{}, wantNoHeaders: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("If-None-Match"); got != tt.wantETag {
					t.Errorf("If-None-Match = %q, want %q", got, tt.wantETag)
				}
				if got := r.Header.Get("If-Modified-Since"); got != tt.wantModified {
					t.Errorf("If-Modified-Since = %q, want %q", got, tt.wantModified)
				}
			}))
			t.Cleanup(server.Close)
			client, err := New(Options{BaseURL: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.ConditionalRequest(context.Background(), "x.get", tt.validator).Get("/"); err != nil {
				t.Fatal(err)
			}
			if tt.validator.IsZero() != tt.wantNoHeaders {
				t.Errorf("IsZero = %v", tt.validator.IsZero())
			}
		})
	}
}

func TestValidatorFromTime(t *testing.T) {
	t.Parallel()
	got := ValidatorFromTime(`"e"`, time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC))
	if got.LastModified != "Mon, 02 Jan 2006 15:04:05 GMT" || got.ETag != `"e"` {
		t.Fatalf("validator = %+v", got)
	}
	if ValidatorFromTime("", time.Time{}).LastModified != "" {
		t.Fatal("zero time should not set LastModified")
	}
}

func TestNotModifiedIsDistinctNonError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
		_, _ = w.Write([]byte(`{"name":"first"}`))
	}))
	t.Cleanup(server.Close)
	client, err := New(Options{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	var first struct{ Name string }
	response, err := client.Request(context.Background(), "x.get").SetResult(&first).ForceContentType("application/json").Get("/")
	if err != nil || NotModified(response) || first.Name != "first" {
		t.Fatalf("first: err=%v notModified=%v result=%+v", err, NotModified(response), first)
	}
	validator := ValidatorFrom(response)
	if validator.ETag != `"v1"` || validator.LastModified == "" {
		t.Fatalf("validator = %+v", validator)
	}

	second := struct{ Name string }{Name: "cached"}
	response, err = client.ConditionalRequest(context.Background(), "x.get", validator).SetResult(&second).Get("/")
	if err != nil {
		t.Fatalf("304 returned error: %v", err)
	}
	if !NotModified(response) || len(response.Body()) != 0 || second.Name != "cached" {
		t.Fatalf("304: notModified=%v body=%q result=%+v", NotModified(response), response.Body(), second)
	}
	if NotModified(nil) || (ValidatorFrom(nil) != Validator{}) {
		t.Fatal("nil response helpers")
	}
}

func TestResponseHookAndRateLimitDetails(t *testing.T) {
	t.Parallel()

	reset := time.Now().Add(time.Hour).Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Limit", "60")
		w.Header().Set("X-Ratelimit-Remaining", "0")
		w.Header().Set("X-Ratelimit-Used", "60")
		w.Header().Set("X-Ratelimit-Resource", "core")
		w.Header().Set("X-Ratelimit-Reset", strconv.FormatInt(reset.Unix(), 10))
		switch r.URL.Path {
		case "/ok":
		case "/nm":
			w.WriteHeader(http.StatusNotModified)
		case "/429":
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
		case "/500":
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	var mu sync.Mutex
	var infos []ResponseInfo
	client, err := New(Options{BaseURL: server.URL, OnResponse: func(_ context.Context, info ResponseInfo) {
		mu.Lock()
		defer mu.Unlock()
		infos = append(infos, info)
	}})
	if err != nil {
		t.Fatal(err)
	}

	var limited *ResponseError
	for _, path := range []string{"/ok", "/nm", "/429", "/500"} {
		_, err := client.Request(context.Background(), "x.get").Get(path)
		if path == "/429" {
			if !errors.As(err, &limited) {
				t.Fatalf("429 error = %v", err)
			}
		}
	}
	if len(infos) != 4 {
		t.Fatalf("hook calls = %d", len(infos))
	}
	for i, want := range []int{200, 304, 429, 500} {
		if infos[i].StatusCode != want || infos[i].Header.Get("X-Ratelimit-Limit") != "60" {
			t.Errorf("info[%d] = %+v", i, infos[i])
		}
		if r := infos[i].RateLimit; r.Remaining == nil || *r.Remaining != 0 || r.Resource != "core" || !r.Reset.Equal(reset) {
			t.Errorf("info[%d] rate limit = %+v", i, r)
		}
	}
	infos[0].Header.Set("X-Ratelimit-Limit", "mutated")

	if limited.RetryAfter != 30*time.Second || limited.RateLimit.Limit == nil || *limited.RateLimit.Limit != 60 ||
		limited.RateLimit.Used == nil || *limited.RateLimit.Used != 60 || !limited.RateLimit.Reset.Equal(reset) {
		t.Fatalf("ResponseError = %+v", limited)
	}
	if infos[2].RetryAfter != 30*time.Second {
		t.Errorf("info retry-after = %v", infos[2].RetryAfter)
	}
}

func TestHookNotCalledOnTransportFailureAndPanicRecovered(t *testing.T) {
	t.Parallel()

	var calls int
	client, err := New(Options{BaseURL: "http://127.0.0.1:1", OnResponse: func(context.Context, ResponseInfo) { calls++ }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Request(context.Background(), "x.get").Get("/"); err == nil {
		t.Fatal("expected transport error")
	}
	if calls != 0 {
		t.Fatalf("hook calls = %d", calls)
	}

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(server.Close)
	client, err = New(Options{BaseURL: server.URL, OnResponse: func(context.Context, ResponseInfo) { panic("boom") }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Request(context.Background(), "x.get").Get("/"); err != nil {
		t.Fatalf("panic leaked: %v", err)
	}
}

func TestHookRunsPerRetryAttempt(t *testing.T) {
	t.Parallel()

	var served int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served++
		if served == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	t.Cleanup(server.Close)
	var calls int
	client, err := New(Options{
		BaseURL:    server.URL,
		Retry:      &RetryOptions{Count: 1, WaitTime: time.Millisecond, MaxWaitTime: time.Millisecond},
		OnResponse: func(context.Context, ResponseInfo) { calls++ },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Request(context.Background(), "x.get").Get("/"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("hook calls = %d, want 2", calls)
	}
}

func TestParseRetryAfter(t *testing.T) {
	fixed := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	old := now
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = old })

	tests := []struct {
		value string
		want  time.Duration
	}{
		{"30", 30 * time.Second},
		{" 5 ", 5 * time.Second},
		{fixed.Add(90 * time.Second).Format(http.TimeFormat), 90 * time.Second},
		{fixed.Add(-time.Hour).Format(http.TimeFormat), 0},
		{"-5", 0},
		{"soon", 0},
		{"", 0},
		{"99999999999999999999", 0},
	}
	for _, tt := range tests {
		if got := parseRetryAfter(tt.value); got != tt.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestParseRateLimit(t *testing.T) {
	fixed := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	old := now
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = old })

	got := parseRateLimit(http.Header{"Ratelimit-Limit": {"100"}, "Ratelimit-Remaining": {"7"}, "Ratelimit-Reset": {"10"}})
	if got.Limit == nil || *got.Limit != 100 || got.Remaining == nil || *got.Remaining != 7 || !got.Reset.Equal(fixed.Add(10*time.Second)) {
		t.Fatalf("IETF = %+v", got)
	}
	got = parseRateLimit(http.Header{"X-Ratelimit-Remaining": {"nope"}, "X-Ratelimit-Reset": {"bad"}})
	if got.Remaining != nil || got.Limit != nil || !got.Reset.IsZero() {
		t.Fatalf("invalid = %+v", got)
	}
	if got := parseRateLimit(http.Header{}); got.Remaining != nil || !got.Reset.IsZero() {
		t.Fatalf("missing = %+v", got)
	}
}
