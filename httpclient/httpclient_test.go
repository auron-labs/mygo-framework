package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientJSONConfiguration(t *testing.T) {
	t.Parallel()

	type payload struct {
		Name string `json:"name"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/widgets" || request.Header.Get("X-Client") != "test" {
			t.Errorf("request = %s %s, X-Client %q", request.Method, request.URL.Path, request.Header.Get("X-Client"))
		}
		if request.Header.Get("Accept") != "application/json" || request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("JSON headers = %v", request.Header)
		}
		var input payload
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input.Name != "sent" {
			t.Errorf("request body = %+v, %v", input, err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"name":"received"}`)
	}))
	t.Cleanup(server.Close)

	client, err := New(Options{
		BaseURL: server.URL,
		Timeout: time.Second,
		Headers: map[string]string{"X-Client": "test"},
		Logger:  slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}

	var output payload
	response, err := client.Request(context.Background(), "widgets.create").
		SetBody(payload{Name: "sent"}).
		SetResult(&output).
		Post("/widgets")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode() != http.StatusOK || output.Name != "received" {
		t.Fatalf("response = %d, %+v", response.StatusCode(), output)
	}
	if client.Resty().RetryCount != 0 || client.Resty().GetClient().Timeout != time.Second {
		t.Fatalf("retry count = %d, timeout = %s", client.Resty().RetryCount, client.Resty().GetClient().Timeout)
	}
}

func TestResponseErrorAndSafeMetric(t *testing.T) {
	t.Parallel()

	const (
		authorization = "Bearer do-not-log"
		responseBody  = "private response body"
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != authorization {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		writer.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(writer, responseBody)
	}))
	t.Cleanup(server.Close)

	var output bytes.Buffer
	client, err := New(Options{
		BaseURL: server.URL,
		Headers: map[string]string{"Authorization": authorization},
		Logger:  slog.New(slog.NewTextHandler(&output, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.Request(context.Background(), "widgets.update").Put("/private/42")
	if response == nil {
		t.Fatal("missing HTTP response")
	}
	var responseError *ResponseError
	if !errors.As(err, &responseError) || responseError.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("error = %v", err)
	}

	logged := output.String()
	for _, want := range []string{"operation=widgets.update", "host=" + strings.TrimPrefix(server.URL, "http://"), "status=422", "duration="} {
		if !strings.Contains(logged, want) {
			t.Errorf("log %q does not contain %q", logged, want)
		}
	}
	for _, secret := range []string{authorization, responseBody, "/private/42"} {
		if strings.Contains(logged, secret) || strings.Contains(err.Error(), secret) {
			t.Errorf("secret %q leaked: log=%q error=%q", secret, logged, err)
		}
	}
}

func TestSuppliedTransport(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://example.test/status" {
			t.Errorf("URL = %q", request.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Request:    request,
		}, nil
	})
	client, err := New(Options{
		BaseURL:   "https://example.test",
		Transport: transport,
		Logger:    slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}

	var result struct {
		OK bool `json:"ok"`
	}
	_, err = client.Request(context.Background(), "status.read").SetResult(&result).Get("/status")
	if err != nil || !result.OK {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
}

func TestInvalidOptions(t *testing.T) {
	t.Parallel()

	for _, options := range []Options{
		{BaseURL: "example.test"},
		{BaseURL: "ftp://example.test"},
		{Timeout: -time.Second},
		{Retry: &RetryOptions{}},
		{Retry: &RetryOptions{Count: MaxRetryCount + 1}},
		{Retry: &RetryOptions{Count: 1, WaitTime: time.Second, MaxWaitTime: time.Millisecond}},
	} {
		if _, err := New(options); err == nil {
			t.Errorf("New(%+v) succeeded", options)
		}
	}
}

func TestTimeoutAndCancellationReachRequest(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		timeout    time.Duration
		cancel     bool
		want       error
		cancelWait time.Duration
	}{
		{name: "client timeout", timeout: 30 * time.Millisecond, want: context.DeadlineExceeded},
		{name: "caller cancellation", timeout: time.Second, cancel: true, want: context.Canceled, cancelWait: 10 * time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			requestCanceled := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
				<-request.Context().Done()
				close(requestCanceled)
			}))
			t.Cleanup(server.Close)

			client, err := New(Options{
				BaseURL: server.URL,
				Timeout: test.timeout,
				Logger:  slog.New(slog.DiscardHandler),
			})
			if err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if test.cancel {
				time.AfterFunc(test.cancelWait, cancel)
			}
			_, err = client.Request(ctx, "slow.read").Get("/slow")
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			select {
			case <-requestCanceled:
			case <-time.After(time.Second):
				t.Fatal("server request context was not canceled")
			}
		})
	}
}

func TestBoundedRetries(t *testing.T) {
	t.Parallel()

	t.Run("reads retry transient status up to the configured count", func(t *testing.T) {
		t.Parallel()
		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			if attempts.Add(1) < 3 {
				writer.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(server.Close)

		client := newRetryClient(t, server.URL, false)
		response, err := client.Request(context.Background(), "status.read").Get("/status")
		if err != nil || response.StatusCode() != http.StatusNoContent || attempts.Load() != 3 {
			t.Fatalf("status = %d, attempts = %d, error = %v", response.StatusCode(), attempts.Load(), err)
		}
	})

	t.Run("writes require explicit opt in", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			name         string
			writes       bool
			wantAttempts int32
			wantStatus   int
		}{
			{name: "disabled", wantAttempts: 1, wantStatus: http.StatusServiceUnavailable},
			{name: "enabled", writes: true, wantAttempts: 2, wantStatus: http.StatusNoContent},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				var attempts atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
					if attempts.Add(1) == 1 {
						writer.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					writer.WriteHeader(http.StatusNoContent)
				}))
				t.Cleanup(server.Close)

				client := newRetryClient(t, server.URL, test.writes)
				response, err := client.Request(context.Background(), "widgets.create").Post("/widgets")
				if response.StatusCode() != test.wantStatus || attempts.Load() != test.wantAttempts {
					t.Fatalf("status = %d, attempts = %d, error = %v", response.StatusCode(), attempts.Load(), err)
				}
				if test.writes && err != nil {
					t.Fatal(err)
				}
				if !test.writes {
					var responseError *ResponseError
					if !errors.As(err, &responseError) {
						t.Fatalf("error = %v", err)
					}
				}
			})
		}
	})
}

func newRetryClient(t *testing.T, baseURL string, writes bool) *Client {
	t.Helper()
	client, err := New(Options{
		BaseURL: baseURL,
		Logger:  slog.New(slog.DiscardHandler),
		Retry: &RetryOptions{
			Count:       2,
			WaitTime:    time.Millisecond,
			MaxWaitTime: 2 * time.Millisecond,
			Writes:      writes,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

var _ http.RoundTripper = roundTripFunc(nil)
