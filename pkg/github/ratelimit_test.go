package github

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRateLimitHeaders(t *testing.T) {
	resetTime := time.Now().Add(time.Hour).Unix()
	resp := &http.Response{
		Header: http.Header{
			"X-RateLimit-Limit":     []string{"5000"},
			"X-RateLimit-Remaining": []string{"4999"},
			"X-RateLimit-Used":      []string{"1"},
			"X-RateLimit-Reset":     []string{strconv.FormatInt(resetTime, 10)},
		},
	}

	info, err := ParseRateLimitHeaders(resp)
	require.NoError(t, err)
	assert.Equal(t, 5000, info.Limit)
	assert.Equal(t, 4999, info.Remaining)
	assert.Equal(t, 1, info.Used)
	assert.Equal(t, time.Unix(resetTime, 0), info.Reset)
}

func TestParseRateLimitHeaders_NilResponse(t *testing.T) {
	_, err := ParseRateLimitHeaders(nil)
	assert.Error(t, err)
}

func TestParseRateLimitHeaders_InvalidHeader(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"X-RateLimit-Limit": []string{"not-a-number"},
		},
	}
	_, err := ParseRateLimitHeaders(resp)
	assert.Error(t, err)
}

func TestIsRateLimited(t *testing.T) {
	tests := []struct {
		name string
		resp *http.Response
		want bool
	}{
		{"nil response", nil, false},
		{"429 status", &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}, true},
		{"403 with remaining 0", &http.Response{
			StatusCode: http.StatusForbidden,
			Header:     http.Header{"X-RateLimit-Remaining": []string{"0"}},
		}, true},
		{"403 without header", &http.Response{
			StatusCode: http.StatusForbidden,
			Header:     http.Header{},
		}, false},
		{"200 ok", &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}, false},
		// Edge case: 403 with remaining > 0 should not be considered rate limited
		{"403 with remaining > 0", &http.Response{
			StatusCode: http.StatusForbidden,
			Header:     http.Header{"X-RateLimit-Remaining": []string{"10"}},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsRateLimited(tt.resp))
		})
	}
}

func TestWaitForRateLimit_ContextCancelled(t *testing.T) {
	info := &RateLimitInfo{Reset: time.Now().Add(10 * time.Second)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := WaitForRateLimit(ctx, info)
	assert.Error(t, err)
}

func TestWaitForRateLimit_AlreadyExpired(t *testing.T) {
	info := &RateLimitInfo{Reset: time.Now().Add(-time.Second)}
	err := WaitForRateLimit(context.Background(), info)
	assert.NoError(t, err)
}
