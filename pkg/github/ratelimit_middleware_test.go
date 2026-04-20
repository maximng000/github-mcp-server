package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimitTransport(t *testing.T) {
	t.Run("wraps base transport", func(t *testing.T) {
		base := http.DefaultTransport
		transport := NewRateLimitTransport(base)
		assert.NotNil(t, transport)
	})

	t.Run("uses default transport when nil", func(t *testing.T) {
		transport := NewRateLimitTransport(nil)
		assert.NotNil(t, transport)
	})
}

func TestRateLimitTransport_RoundTrip(t *testing.T) {
	t.Run("passes through successful response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Limit", "5000")
			w.Header().Set("X-RateLimit-Remaining", "4999")
			w.Header().Set("X-RateLimit-Reset", "1700000000")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		transport := NewRateLimitTransport(nil)
		client := &http.Client{Transport: transport}

		resp, err := client.Get(server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns 429 response without blocking when reset is far future", func(t *testing.T) {
		resetTime := time.Now().Add(2 * time.Second).Unix()
		callCount := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime))
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		transport := NewRateLimitTransport(nil)
		client := &http.Client{Transport: transport}

		resp, err := client.Get(server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should still return the 429 to the caller
		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
		// Ensure the server was only called once (no automatic retry on 429)
		assert.Equal(t, 1, callCount)
	})

	t.Run("forwards request headers", func(t *testing.T) {
		var receivedAuth string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		transport := NewRateLimitTransport(nil)
		client := &http.Client{Transport: transport}

		req, err := http.NewRequest(http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer test-token")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, "Bearer test-token", receivedAuth)
	})
}
