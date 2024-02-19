package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	model "github.com/mattermost/mattermost/server/public/model"
)

type Endpoint struct {
	Path          string
	Method        string
	Execute       func(w http.ResponseWriter, r *http.Request)
	RequiresAdmin bool
}

func ReturnStatusOK(w io.Writer) {
	m := make(map[string]string)
	m[model.STATUS] = model.StatusOk
	_, _ = w.Write([]byte(model.MapToJSON(m)))
}

func verifyHTTPSecret(expected, got string) (status int, err error) {
	for {
		if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1 {
			break
		}

		unescaped, _ := url.QueryUnescape(got)
		if unescaped == got {
			return http.StatusForbidden, errors.New("request URL: secret did not match")
		}
		got = unescaped
	}

	return 0, nil
}

func verifyWebHookSecret(expected, got string, body []byte) (status int, err error) {
	h := hmac.New(sha256.New, []byte(expected))
	_, _ = h.Write(body)
	expectedHash := hex.EncodeToString(h.Sum(nil))

	gotHash := strings.SplitN(got, "=", 2)
	if gotHash[0] != "sha256" {
		return http.StatusForbidden, errors.New("request URL: Not encrypted with sha256 ")
	}

	if expectedHash != gotHash[1] {
		return http.StatusForbidden, errors.New("request URL: secret did not match")
	}

	return http.StatusOK, nil
}
