package github

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultBackoffConfig(t *testing.T) {
	cfg := DefaultBackoffConfig()
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 500*time.Millisecond, cfg.BaseDelay)
	assert.Equal(t, 30*time.Second, cfg.MaxDelay)
	assert.Equal(t, 2.0, cfg.Multiplier)
}

func TestBackoffDelay(t *testing.T) {
	cfg := DefaultBackoffConfig()

	delay0 := BackoffDelay(0, cfg)
	assert.Equal(t, 500*time.Millisecond, delay0)

	delay1 := BackoffDelay(1, cfg)
	assert.Equal(t, 1*time.Second, delay1)

	// Should be capped at MaxDelay
	delayCapped := BackoffDelay(100, cfg)
	assert.Equal(t, cfg.MaxDelay, delayCapped)
}

func TestDoWithBackoff_Success(t *testing.T) {
	calls := 0
	fn := func() (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	resp, err := DoWithBackoff(context.Background(), DefaultBackoffConfig(), fn)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, calls)
}

func TestDoWithBackoff_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := DefaultBackoffConfig()
	cfg.BaseDelay = 200 * time.Millisecond

	calls := 0
	fn := func() (*http.Response, error) {
		calls++
		if calls == 1 {
			cancel()
		}
		return &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}, nil
	}

	_, err := DoWithBackoff(ctx, cfg, fn)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestDoWithBackoff_ExhaustsRetries(t *testing.T) {
	cfg := DefaultBackoffConfig()
	cfg.MaxRetries = 2
	cfg.BaseDelay = 1 * time.Millisecond

	calls := 0
	fn := func() (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}, nil
	}

	resp, err := DoWithBackoff(context.Background(), cfg, fn)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, cfg.MaxRetries+1, calls)
}
