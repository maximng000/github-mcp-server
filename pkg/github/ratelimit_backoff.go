package github

import (
	"context"
	"math"
	"net/http"
	"time"
)

// BackoffConfig holds configuration for exponential backoff retry logic.
type BackoffConfig struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// DefaultBackoffConfig returns a BackoffConfig with sensible defaults.
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		MaxRetries: 5,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
	}
}

// BackoffDelay calculates the delay for a given retry attempt.
func BackoffDelay(attempt int, cfg BackoffConfig) time.Duration {
	delay := float64(cfg.BaseDelay) * math.Pow(cfg.Multiplier, float64(attempt))
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}
	return time.Duration(delay)
}

// DoWithBackoff executes fn with exponential backoff on rate limit (429) or secondary rate limit (403) responses.
func DoWithBackoff(ctx context.Context, cfg BackoffConfig, fn func() (*http.Response, error)) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		resp, err = fn()
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusForbidden {
			return resp, nil
		}

		// Try to honour Retry-After / X-RateLimit-Reset before falling back to backoff.
		info := ParseRateLimitHeaders(resp)
		var wait time.Duration
		if info != nil && IsRateLimited(info) {
			wait = time.Until(info.Reset)
		}
		if wait <= 0 {
			wait = BackoffDelay(attempt, cfg)
		}

		if attempt == cfg.MaxRetries {
			break
		}

		select {
		case <-ctx.Done():
			return resp, ctx.Err()
		case <-time.After(wait):
		}
	}

	return resp, nil
}
