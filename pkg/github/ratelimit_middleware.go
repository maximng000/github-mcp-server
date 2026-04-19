package github

import (
	"context"
	"fmt"
	"net/http"
)

// RateLimitTransport is an http.RoundTripper that handles GitHub rate limiting
// by inspecting responses and optionally waiting for the limit to reset.
type RateLimitTransport struct {
	Base    http.RoundTripper
	WaitOn429 bool
}

// NewRateLimitTransport creates a RateLimitTransport wrapping the given base transport.
func NewRateLimitTransport(base http.RoundTripper, waitOn429 bool) *RateLimitTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &RateLimitTransport{Base: base, WaitOn429: waitOn429}
}

// RoundTrip executes the request, retrying once after waiting if rate limited.
func (t *RateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.Base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if !t.WaitOn429 || !IsRateLimited(resp) {
		return resp, nil
	}

	info, parseErr := ParseRateLimitHeaders(resp)
	if parseErr != nil {
		return resp, nil
	}

	// Drain and close the original response before retrying.
	_ = resp.Body.Close()

	ctx := req.Context()
	if waitErr := WaitForRateLimit(ctx, info); waitErr != nil {
		return nil, fmt.Errorf("rate limit wait cancelled: %w", waitErr)
	}

	// Clone request to allow retry (body may be nil for GET requests).
	retryReq := req.Clone(context.WithoutCancel(ctx))
	return t.Base.RoundTrip(retryReq)
}
