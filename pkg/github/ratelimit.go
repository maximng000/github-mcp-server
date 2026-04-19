package github

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// RateLimitInfo holds parsed rate limit headers from GitHub API responses.
type RateLimitInfo struct {
	Limit     int
	Remaining int
	Reset     time.Time
	Used      int
}

// ParseRateLimitHeaders extracts rate limit information from HTTP response headers.
func ParseRateLimitHeaders(resp *http.Response) (*RateLimitInfo, error) {
	if resp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	info := &RateLimitInfo{}

	if v := resp.Header.Get("X-RateLimit-Limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid X-RateLimit-Limit: %w", err)
		}
		info.Limit = n
	}

	if v := resp.Header.Get("X-RateLimit-Remaining"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid X-RateLimit-Remaining: %w", err)
		}
		info.Remaining = n
	}

	if v := resp.Header.Get("X-RateLimit-Used"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid X-RateLimit-Used: %w", err)
		}
		info.Used = n
	}

	if v := resp.Header.Get("X-RateLimit-Reset"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid X-RateLimit-Reset: %w", err)
		}
		info.Reset = time.Unix(n, 0)
	}

	return info, nil
}

// IsRateLimited returns true if the response indicates rate limiting (HTTP 429 or 403).
func IsRateLimited(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	return resp.StatusCode == http.StatusTooManyRequests ||
		(resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0")
}

// WaitForRateLimit blocks until the rate limit resets or the context is cancelled.
func WaitForRateLimit(ctx context.Context, info *RateLimitInfo) error {
	if info == nil {
		return nil
	}
	waitDuration := time.Until(info.Reset)
	if waitDuration <= 0 {
		return nil
	}
	select {
	case <-time.After(waitDuration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
