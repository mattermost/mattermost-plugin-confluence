package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostForgeReset(t *testing.T) {
	const secret = "test-shared-secret-32-chars-len!"

	t.Run("happy path returns queuedDeleted", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			expected := hex.EncodeToString(mac.Sum(nil))
			assert.Equal(t, expected, r.Header.Get("X-MM-Signature"), "plugin must sign the body with the current secret")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "queuedDeleted": 7})
		}))
		defer server.Close()

		deleted, err := postForgeReset(server.URL, secret)
		require.NoError(t, err)
		assert.Equal(t, 7, deleted)
	})

	t.Run("403 maps to drift-recovery instructions", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"invalid signature"}`, http.StatusForbidden)
		}))
		defer server.Close()

		_, err := postForgeReset(server.URL, secret)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "wipeRegistrationFn")
		assert.Contains(t, err.Error(), "install cloud")
	})

	t.Run("200 with ok=false is rejected", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":false}`))
		}))
		defer server.Close()

		_, err := postForgeReset(server.URL, secret)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ok=false")
	})

	t.Run("200 with unparseable body is rejected", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`not json`))
		}))
		defer server.Close()

		_, err := postForgeReset(server.URL, secret)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unparseable")
	})

	t.Run("500 surfaces upstream body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer server.Close()

		_, err := postForgeReset(server.URL, secret)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500")
		assert.Contains(t, err.Error(), "boom")
	})

	t.Run("refuses to follow redirects", func(t *testing.T) {
		// A malicious bridge that redirects elsewhere should not cause the
		// signed body to be replayed against the redirect target.
		called := 0
		dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called++
			w.WriteHeader(http.StatusOK)
		}))
		defer dst.Close()
		src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Redirect(w, &http.Request{}, dst.URL, http.StatusTemporaryRedirect)
		}))
		defer src.Close()

		_, err := postForgeReset(src.URL, secret)
		require.Error(t, err)
		assert.Zero(t, called, "redirect target must not be hit")
	})
}

func TestPostForgeRegisterParsesURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"urls": map[string]string{
				"drain":    "https://x.webtrigger.atlassian.app/public/drain",
				"register": "https://x.webtrigger.atlassian.app/public/register",
				"reset":    "https://x.webtrigger.atlassian.app/public/reset",
			},
		})
	}))
	defer server.Close()

	urls, err := postForgeRegister(server.URL, "secret")
	require.NoError(t, err)
	require.NotNil(t, urls)
	assert.Equal(t, "https://x.webtrigger.atlassian.app/public/reset", urls.Reset)
	assert.Equal(t, "https://x.webtrigger.atlassian.app/public/drain", urls.Drain)
	assert.Equal(t, "https://x.webtrigger.atlassian.app/public/register", urls.Register)
}

func TestPostForgeRegisterToleratesOldBridge(t *testing.T) {
	// Old bridge responses returned just {"ok":true} with no urls field;
	// the plugin must not crash and must return nil-or-zero URLs.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	urls, err := postForgeRegister(server.URL, "secret")
	require.NoError(t, err)
	require.NotNil(t, urls)
	assert.Empty(t, urls.Reset)
}
