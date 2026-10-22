// Package httpclient configures Resty for outbound application requests.
package httpclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// DefaultTimeout bounds requests when the caller does not configure a timeout.
const DefaultTimeout = 30 * time.Second

// Options are the shared settings for an outbound HTTP client.
type Options struct {
	BaseURL   string
	Timeout   time.Duration
	Headers   map[string]string
	Transport http.RoundTripper
	Logger    *slog.Logger
}

// Client adds framework defaults while preserving access to Resty.
type Client struct {
	resty  *resty.Client
	logger *slog.Logger
}

// ResponseError reports an unsuccessful HTTP status without exposing its body.
type ResponseError struct {
	Operation  string
	Method     string
	Host       string
	StatusCode int
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("%s: %s %s returned HTTP %d", e.Operation, e.Method, e.Host, e.StatusCode)
}

type operationKey struct{}

// New creates a JSON Resty client with retries disabled.
func New(options Options) (*Client, error) {
	if options.Timeout < 0 {
		return nil, errors.New("http client timeout must not be negative")
	}
	if err := validateBaseURL(options.BaseURL); err != nil {
		return nil, err
	}

	timeout := options.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}

	restyClient := resty.New().
		SetTimeout(timeout).
		SetRetryCount(0).
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		})
	if options.BaseURL != "" {
		restyClient.SetBaseURL(options.BaseURL)
	}
	if len(options.Headers) > 0 {
		restyClient.SetHeaders(options.Headers)
	}
	if options.Transport != nil {
		restyClient.SetTransport(options.Transport)
	}

	client := &Client{resty: restyClient, logger: logger}
	restyClient.OnAfterResponse(client.afterResponse)
	restyClient.OnError(client.afterError)
	return client, nil
}

// Request creates a Resty request carrying its context and metric operation.
func (c *Client) Request(ctx context.Context, operation string) *resty.Request {
	if strings.TrimSpace(operation) == "" {
		operation = "http.request"
	}
	return c.resty.R().SetContext(context.WithValue(ctx, operationKey{}, operation))
}

// Resty returns the configured underlying client.
func (c *Client) Resty() *resty.Client {
	return c.resty
}

func (c *Client) afterResponse(_ *resty.Client, response *resty.Response) error {
	c.record(response.Request, response.StatusCode(), response.Time())
	if !response.IsError() {
		return nil
	}

	return &ResponseError{
		Operation:  operation(response.Request),
		Method:     response.Request.Method,
		Host:       host(response.Request),
		StatusCode: response.StatusCode(),
	}
}

func (c *Client) afterError(request *resty.Request, err error) {
	var responseError *resty.ResponseError
	if errors.As(err, &responseError) && responseError.Response != nil {
		if responseError.Response.RawResponse != nil {
			return // afterResponse already recorded an HTTP response.
		}
		c.record(request, 0, responseError.Response.Time())
		return
	}

	c.record(request, 0, 0)
}

func (c *Client) record(request *resty.Request, status int, duration time.Duration) {
	c.logger.LogAttrs(request.Context(), slog.LevelInfo, "http request",
		slog.String("operation", operation(request)),
		slog.String("host", host(request)),
		slog.Int("status", status),
		slog.Duration("duration", duration),
	)
}

func operation(request *resty.Request) string {
	value, _ := request.Context().Value(operationKey{}).(string)
	if value == "" {
		return "http.request"
	}
	return value
}

func host(request *resty.Request) string {
	if request.RawRequest != nil && request.RawRequest.URL != nil {
		return request.RawRequest.URL.Host
	}
	parsed, err := url.Parse(request.URL)
	if err != nil {
		return ""
	}
	return parsed.Host
}

func validateBaseURL(value string) error {
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("invalid HTTP client base URL %q", value)
	}
	return nil
}
