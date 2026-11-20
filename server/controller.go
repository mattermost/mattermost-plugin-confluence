package main

import (
	"crypto/subtle"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime/debug"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

type Endpoint struct {
	Path            string
	Method          string
	Execute         func(w http.ResponseWriter, r *http.Request, p *Plugin)
	IsAuthenticated bool
}

// Endpoints is a map of endpoint key to endpoint object
// Usage: getEndpointKey(GetMetadata): GetMetadata
var Endpoints = map[string]*Endpoint{
	getEndpointKey(confluenceCloudWebhook):              confluenceCloudWebhook,
	getEndpointKey(saveChannelSubscription):             saveChannelSubscription,
	getEndpointKey(editChannelSubscription):             editChannelSubscription,
	getEndpointKey(confluenceServerWebhook):             confluenceServerWebhook,
	getEndpointKey(getChannelSubscription):              getChannelSubscription,
	getEndpointKey(autocompleteGetChannelSubscriptions): autocompleteGetChannelSubscriptions,
	getEndpointKey(userConnect):                         userConnect,
	getEndpointKey(userConnectComplete):                 userConnectComplete,
	getEndpointKey(userConnectionInfo):                  userConnectionInfo,
	getEndpointKey(getPluginConfig):                     getPluginConfig,
}

// Uniquely identifies an endpoint using path and method
func getEndpointKey(endpoint *Endpoint) string {
	return util.GetKeyHash(endpoint.Path + "-" + endpoint.Method)
}

// InitAPI initializes the REST API
func (p *Plugin) InitAPI() *mux.Router {
	r := mux.NewRouter()
	handleStaticFiles(r)

	s := r.PathPrefix("/api/v1").Subrouter()
	for _, endpoint := range Endpoints {
		handler := endpoint.Execute
		if endpoint.IsAuthenticated {
			s.HandleFunc(endpoint.Path, p.checkAuth(p.wrapHandler(handler))).Methods(endpoint.Method)
		} else {
			s.HandleFunc(endpoint.Path, p.wrapHandler(handler)).Methods(endpoint.Method)
		}
	}

	return r
}

func (p *Plugin) checkAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get(config.HeaderMattermostUserID)
		if userID == "" {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}
}

// wrapHandler ensures the plugin is passed to the handler
func (p *Plugin) wrapHandler(handler func(http.ResponseWriter, *http.Request, *Plugin)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if x := recover(); x != nil {
				// Query parameters are intentionally excluded from logs.
				p.client.Log.Error("Recovered from a panic",
					"path", r.URL.Path,
					"error", x,
					"stack", string(debug.Stack()))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()

		handler(w, r, p)
	}
}

// handleStaticFiles handles the static files under the assets directory.
func handleStaticFiles(r *mux.Router) {
	bundlePath, err := config.Mattermost.GetBundlePath()
	if err != nil {
		config.Mattermost.LogWarn("Failed to get bundle path.", "Error", err.Error())
		return
	}

	// This will serve static files from the 'assets' directory under '/static/<filename>'
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(bundlePath, "assets")))))
}

// IsAdmin verifies if provided request is performed by a logged-in Mattermost user.
func IsAdmin(w http.ResponseWriter, r *http.Request) bool {
	userID := r.Header.Get(config.HeaderMattermostUserID)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	return util.IsSystemAdmin(userID)
}

func ReturnStatusOK(w io.Writer) {
	m := make(map[string]string)
	m[model.STATUS] = model.StatusOk
	_, _ = w.Write([]byte(model.MapToJSON(m)))
}

const maxSecretDecodeDepth = 3

func verifyHTTPSecret(expected, got string) (status int, err error) {
	// The provided 'got' string is compared to the 'expected' string using a
	// constant-time comparison to prevent timing attacks. If 'got' does not match,
	// it is unescaped at most maxSecretDecodeDepth times. Values longer than the
	// longest possible encoding of 'expected' at that depth (each escaping pass
	// expands a byte to at most 3 bytes) are rejected before any decoding.
	errMismatch := errors.New("request URL: secret did not match")
	if expected == "" {
		return http.StatusForbidden, errMismatch
	}

	maxLen := len(expected)
	for i := 0; i < maxSecretDecodeDepth; i++ {
		maxLen *= 3
	}
	if len(got) > maxLen {
		return http.StatusForbidden, errMismatch
	}

	for i := 0; ; i++ {
		if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1 {
			return 0, nil
		}
		if i == maxSecretDecodeDepth {
			return http.StatusForbidden, errMismatch
		}
		unescaped, unescapeErr := url.QueryUnescape(got)
		if unescapeErr != nil || unescaped == got {
			return http.StatusForbidden, errMismatch
		}
		got = unescaped
	}
}
