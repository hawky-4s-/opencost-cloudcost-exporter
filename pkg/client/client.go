// Package client provides an HTTP client for the OpenCost cloudCost API.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/hawky-4s-/opencost-cloudcost-exporter/pkg/types"
)

// Client is an HTTP client for the OpenCost cloudCost API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	window     string
	aggregate  string
	maxRetries int
}

// Option is a functional option for configuring the Client.
type Option func(*Client)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithWindow sets the time window for cost queries.
func WithWindow(window string) Option {
	return func(c *Client) {
		c.window = window
	}
}

// WithAggregate sets the aggregation dimensions.
func WithAggregate(aggregate string) Option {
	return func(c *Client) {
		c.aggregate = aggregate
	}
}

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(retries int) Option {
	return func(c *Client) {
		c.maxRetries = retries
	}
}

// New creates a new OpenCost API client.
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		window:     "1d",
		aggregate:  "service,category",
		maxRetries: 3,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// FetchCloudCosts fetches cloud cost data from the OpenCost API with retry support.
func (c *Client) FetchCloudCosts(ctx context.Context) (*types.CloudCostResponse, error) {
	endpoint, err := url.JoinPath(c.baseURL, "/cloudCost")
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Build query parameters
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}

	q := u.Query()
	q.Set("window", c.window)
	//q.Set("aggregate", c.aggregate)
	u.RawQuery = q.Encode()

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s...
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			slog.Warn("retrying OpenCost API request",
				"attempt", attempt,
				"max_retries", c.maxRetries,
				"backoff", backoff.String(),
				"last_error", lastErr.Error(),
			)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		result, err := c.doFetch(ctx, u.String())
		if err == nil {
			return result, nil
		}
		lastErr = err

		// Don't retry on context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("after %d retries: %w", c.maxRetries, lastErr)
}

func (c *Client) doFetch(ctx context.Context, url string) (*types.CloudCostResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	slog.Debug("sending HTTP request",
		"method", req.Method,
		"url", url,
		"headers", req.Header,
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Debug("HTTP request failed",
			"method", req.Method,
			"url", url,
			"error", err,
		)
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Read body for logging and parsing
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Log response details at debug level
	bodyPreview := string(body)
	if len(bodyPreview) > 500 {
		bodyPreview = bodyPreview[:500] + "... (truncated)"
	}
	slog.Debug("received HTTP response",
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"content_length", resp.ContentLength,
		"headers", resp.Header,
		"body_preview", bodyPreview,
	)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result types.CloudCostResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// Ping checks if the OpenCost API is reachable.
func (c *Client) Ping(ctx context.Context) error {
	endpoint, err := url.JoinPath(c.baseURL, "/healthz")
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	slog.Debug("sending HTTP request",
		"method", req.Method,
		"url", endpoint,
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Debug("HTTP request failed",
			"method", req.Method,
			"url", endpoint,
			"error", err,
		)
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("received HTTP response",
		"status_code", resp.StatusCode,
		"status", resp.Status,
	)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// DefaultExchangeRateURL is the default Frankfurter API endpoint.
const DefaultExchangeRateURL = "https://api.frankfurter.dev/v1/latest"

// FetchExchangeRates fetches currency exchange rates from the Frankfurter API.
func (c *Client) FetchExchangeRates(ctx context.Context, base string, symbols []string) (*types.ExchangeRateResponse, error) {
	u, err := url.Parse(DefaultExchangeRateURL)
	if err != nil {
		return nil, fmt.Errorf("parse exchange rate URL: %w", err)
	}

	q := u.Query()
	q.Set("base", base)
	if len(symbols) > 0 {
		symbolStr := ""
		for i, s := range symbols {
			if i > 0 {
				symbolStr += ","
			}
			symbolStr += s
		}
		q.Set("symbols", symbolStr)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	slog.Debug("sending HTTP request",
		"method", req.Method,
		"url", u.String(),
		"headers", req.Header,
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Debug("HTTP request failed",
			"method", req.Method,
			"url", u.String(),
			"error", err,
		)
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	bodyPreview := string(body)
	if len(bodyPreview) > 500 {
		bodyPreview = bodyPreview[:500] + "... (truncated)"
	}
	slog.Debug("received HTTP response",
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"content_length", resp.ContentLength,
		"headers", resp.Header,
		"body_preview", bodyPreview,
	)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result types.ExchangeRateResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	slog.Debug("parsed exchange rates",
		"base", result.Base,
		"date", result.Date,
		"rates", result.Rates,
	)

	return &result, nil
}
