package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

// specialChars is a 32-byte value made of characters that QueryEscape rewrites.
const specialChars = "a b+c/d=e&f%g?h#i;j:k@l$m,n!o~p^"

// secretCheckTimeout bounds how long a single secret verification may take.
// A bounded implementation finishes in microseconds; the unbounded decode loop
// takes several seconds on the payloads below.
const secretCheckTimeout = 500 * time.Millisecond

type secretResult struct {
	status int
	err    error
}

// runVerifyHTTPSecretWithin runs verifyHTTPSecret and fails the test if it does
// not return within the timeout.
func runVerifyHTTPSecretWithin(t *testing.T, expected, got string, timeout time.Duration) (secretResult, bool) {
	t.Helper()

	done := make(chan secretResult, 1)
	start := time.Now()
	go func() {
		status, err := verifyHTTPSecret(expected, got)
		done <- secretResult{status: status, err: err}
	}()

	select {
	case res := <-done:
		return res, true
	case <-time.After(timeout):
		t.Errorf("verifyHTTPSecret did not return within %v for a %d byte value", timeout, len(got))
	}

	// Wait a bit longer only to report what the slow path eventually did.
	select {
	case res := <-done:
		t.Logf("verifyHTTPSecret eventually returned after %v: status=%d err=%v", time.Since(start), res.status, res.err)
	case <-time.After(30 * time.Second):
		t.Logf("verifyHTTPSecret still running after %v", time.Since(start))
	}
	return secretResult{}, false
}

// nestedPercent25 builds "%25" followed by n repetitions of "25". Each URL
// decode pass only strips two bytes from it.
func nestedPercent25(n int) string {
	return "%25" + strings.Repeat("25", n)
}

func encodeTimes(s string, n int) string {
	for i := 0; i < n; i++ {
		s = url.QueryEscape(s)
	}
	return s
}

func TestVerifyHTTPSecretCorrectness(t *testing.T) {
	const special = "a b+c/d=e&f%g"

	for name, tc := range map[string]struct {
		expected string
		got      string
		accepted bool
	}{
		"exact match":                       {expected: testWebhookKey, got: testWebhookKey, accepted: true},
		"exact match with special chars":    {expected: special, got: special, accepted: true},
		"once-encoded special chars":        {expected: special, got: encodeTimes(special, 1), accepted: true},
		"twice-encoded special chars":       {expected: special, got: encodeTimes(special, 2), accepted: true},
		"wrong secret":                      {expected: testWebhookKey, got: "wrong", accepted: false},
		"encoded wrong secret":              {expected: special, got: encodeTimes("a b+c/d=e&f%h", 2), accepted: false},
		"prefix of secret":                  {expected: testWebhookKey, got: testWebhookKey[:len(testWebhookKey)-1], accepted: false},
		"secret with suffix":                {expected: testWebhookKey, got: testWebhookKey + "x", accepted: false},
		"empty got with non-empty expected": {expected: testWebhookKey, got: "", accepted: false},
	} {
		t.Run(name, func(t *testing.T) {
			res, ok := runVerifyHTTPSecretWithin(t, tc.expected, tc.got, secretCheckTimeout)
			require.True(t, ok, "verifyHTTPSecret must return within %v", secretCheckTimeout)

			if tc.accepted {
				assert.NoError(t, res.err)
				assert.Equal(t, 0, res.status)
				return
			}
			assert.Error(t, res.err)
			assert.Equal(t, http.StatusForbidden, res.status)
		})
	}
}

func TestVerifyHTTPSecretRejectsNestedEncodingQuickly(t *testing.T) {
	// ~100,000 bytes; the unbounded loop needs ~50,000 decode passes.
	got := nestedPercent25(50000)

	res, ok := runVerifyHTTPSecretWithin(t, testWebhookKey, got, secretCheckTimeout)
	require.True(t, ok, "verifyHTTPSecret must return within %v", secretCheckTimeout)
	assert.Error(t, res.err)
	assert.Equal(t, http.StatusForbidden, res.status)
}

func TestVerifyHTTPSecretRejectsExcessiveEncodingDepth(t *testing.T) {
	const value = "a b+c/d=e&f%g"

	for _, depth := range []int{10, 20} {
		got := encodeTimes(value, depth)
		res, ok := runVerifyHTTPSecretWithin(t, value, got, secretCheckTimeout)
		require.True(t, ok, "verifyHTTPSecret must return within %v", secretCheckTimeout)
		assert.Error(t, res.err, "secret encoded %d times must be rejected", depth)
		assert.Equal(t, http.StatusForbidden, res.status, "secret encoded %d times must be rejected", depth)
	}
}

func TestVerifyHTTPSecretRejectsOverlongValue(t *testing.T) {
	expected := "0123456789abcdef0123456789abcdeA"
	require.Len(t, expected, 32)

	// The last character 'A' (%41) is wrapped in ~50,000 levels of "%25", so
	// the value is 100,000 bytes yet fully decodes to the expected secret.
	got := expected[:31] + "%" + strings.Repeat("25", 49983) + "41"
	require.Len(t, got, 100000)

	res, ok := runVerifyHTTPSecretWithin(t, expected, got, secretCheckTimeout)
	require.True(t, ok, "verifyHTTPSecret must return within %v", secretCheckTimeout)
	assert.Error(t, res.err)
	assert.Equal(t, http.StatusForbidden, res.status)
}

func TestConfluenceWebhookRejectsNestedEncodedSecretQuickly(t *testing.T) {
	// r.FormValue decodes once, so the handler sees nestedPercent25(50000).
	form := url.Values{"secret": {nestedPercent25(50000)}}.Encode()

	for name, path := range map[string]string{
		"server webhook": "/api/v1/server/webhook",
		"cloud webhook":  "/api/v1/cloud/page_created",
	} {
		t.Run(name, func(t *testing.T) {
			router := setupWebhookRouter(t, true)

			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			done := make(chan struct{})
			go func() {
				router.ServeHTTP(w, req)
				close(done)
			}()

			select {
			case <-done:
				assert.Equal(t, http.StatusForbidden, w.Code)
			case <-time.After(secretCheckTimeout):
				t.Errorf("%s did not respond within %v", path, secretCheckTimeout)
			}
		})
	}
}

// hexEncodeTimes percent-encodes every byte n times. Each pass triples the
// length, which is the largest expansion a single decode pass can undo.
func hexEncodeTimes(s string, n int) string {
	const hexDigits = "0123456789ABCDEF"
	for i := 0; i < n; i++ {
		var b strings.Builder
		for j := 0; j < len(s); j++ {
			b.WriteByte('%')
			b.WriteByte(hexDigits[s[j]>>4])
			b.WriteByte(hexDigits[s[j]&0x0F])
		}
		s = b.String()
	}
	return s
}

func TestVerifyHTTPSecretEdgeCases(t *testing.T) {
	const multiByte = "pässwörd-€-日本語-😀"

	for name, tc := range map[string]struct {
		expected string
		got      string
		accepted bool
	}{
		"query-escaped 3 times":                            {expected: specialChars, got: encodeTimes(specialChars, 3), accepted: true},
		"query-escaped 4 times":                            {expected: specialChars, got: encodeTimes(specialChars, 4), accepted: false},
		"fully hex-encoded 3 times, at the length cap":     {expected: testWebhookKey, got: hexEncodeTimes(testWebhookKey, 3), accepted: true},
		"fully hex-encoded 4 times":                        {expected: testWebhookKey, got: hexEncodeTimes(testWebhookKey, 4), accepted: false},
		"one-byte expected, hex-encoded 3 times":           {expected: "x", got: hexEncodeTimes("x", 3), accepted: true},
		"one-byte expected, query-escaped 3 times":         {expected: "%", got: encodeTimes("%", 3), accepted: true},
		"one-byte expected, one byte over the cap":         {expected: "x", got: hexEncodeTimes("x", 3) + "x", accepted: false},
		"multi-byte UTF-8 query-escaped 3 times":           {expected: multiByte, got: encodeTimes(multiByte, 3), accepted: true},
		"multi-byte UTF-8 hex-encoded 3 times":             {expected: multiByte, got: hexEncodeTimes(multiByte, 3), accepted: true},
		"plus in secret sent escaped":                      {expected: "a+b", got: "a%2Bb", accepted: true},
		"plus in secret sent unescaped decodes to a space": {expected: "a+b", got: "a b", accepted: false},
		"space in secret sent as plus":                     {expected: "a b", got: "a+b", accepted: true},
		"literal invalid escape in secret":                 {expected: "100%zz", got: "100%zz", accepted: true},
		"literal invalid escape in secret, encoded once":   {expected: "100%zz", got: "100%25zz", accepted: true},
		"invalid percent sequence":                         {expected: testWebhookKey, got: "%zz" + testWebhookKey, accepted: false},
		"trailing lone percent":                            {expected: testWebhookKey, got: testWebhookKey + "%", accepted: false},
		"empty expected and empty got":                     {expected: "", got: "", accepted: false},
		"empty expected and non-empty got":                 {expected: "", got: "anything", accepted: false},
	} {
		t.Run(name, func(t *testing.T) {
			res, ok := runVerifyHTTPSecretWithin(t, tc.expected, tc.got, secretCheckTimeout)
			require.True(t, ok, "verifyHTTPSecret must return within %v", secretCheckTimeout)

			if tc.accepted {
				assert.NoError(t, res.err)
				assert.Equal(t, 0, res.status)
				return
			}
			assert.Error(t, res.err)
			assert.Equal(t, http.StatusForbidden, res.status)
		})
	}
}

func TestConfluenceServerWebhookAcceptsAdvertisedURL(t *testing.T) {
	generated, err := generateRandomKey(32)
	require.NoError(t, err)

	for name, value := range map[string]string{
		"generated secret":    generated,
		"special-char secret": specialChars,
		"multi-byte secret":   "ü€😀-0123456789abcdefghijk",
	} {
		t.Run(name, func(t *testing.T) {
			router := setupWebhookRouter(t, true)
			cfg := *config.GetConfig()
			cfg.Secret = value
			config.SetConfig(&cfg)

			advertised := util.GetConfluenceServerWebhookURLPath()
			reencoded := "/server/webhook?secret=" + url.QueryEscape(url.QueryEscape(value))

			for _, path := range []string{advertised, reencoded} {
				req := httptest.NewRequest(http.MethodPost, "/api/v1"+path, strings.NewReader(`{"test":true}`))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code, "path %q", path)
			}
		})
	}
}

func TestConfluenceWebhookRejectsOverlongQuerySecretQuickly(t *testing.T) {
	overlongButMatching := testWebhookKey[:len(testWebhookKey)-1] + "%" + strings.Repeat("25", 49983) + fmt.Sprintf("%X", testWebhookKey[len(testWebhookKey)-1])

	for name, path := range map[string]string{
		"server webhook": "/api/v1/server/webhook",
		"cloud webhook":  "/api/v1/cloud/page_created",
	} {
		for valueName, value := range map[string]string{
			"nested percent":        nestedPercent25(50000),
			"overlong but matching": overlongButMatching,
		} {
			t.Run(name+" "+valueName, func(t *testing.T) {
				router := setupWebhookRouter(t, true)

				req := httptest.NewRequest(http.MethodPost, path+"?secret="+url.QueryEscape(value), strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()

				done := make(chan struct{})
				go func() {
					router.ServeHTTP(w, req)
					close(done)
				}()

				select {
				case <-done:
					assert.Equal(t, http.StatusForbidden, w.Code)
				case <-time.After(secretCheckTimeout):
					t.Errorf("%s did not respond within %v", path, secretCheckTimeout)
				}
			})
		}
	}
}
