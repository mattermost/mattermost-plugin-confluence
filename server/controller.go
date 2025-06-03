package main

import (
	"crypto/subtle"
	"io"
	"net/http"
	"net/url"
	"path/filepath"

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
	getEndpointKey(atlassianConnectJSON):                atlassianConnectJSON,
	getEndpointKey(confluenceCloudWebhook):              confluenceCloudWebhook,
	getEndpointKey(saveChannelSubscription):             saveChannelSubscription,
	getEndpointKey(editChannelSubscription):             editChannelSubscription,
	getEndpointKey(confluenceServerWebhook):             confluenceServerWebhook,
	getEndpointKey(getChannelSubscription):              getChannelSubscription,
	getEndpointKey(autocompleteGetChannelSubscriptions): autocompleteGetChannelSubscriptions,
	getEndpointKey(userConnect):                         userConnect,
	getEndpointKey(userConnectComplete):                 userConnectComplete,
	getEndpointKey(userConnectionInfo):                  userConnectionInfo,
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

func verifyHTTPSecret(expected, got string) (status int, err error) {
	// The loop ensures that the provided 'got' string matches the 'expected' string
	// using a constant-time comparison to prevent timing attacks. If 'got' does not
	// match, it is repeatedly unescaped until it either matches or cannot be further
	// unescaped, at which point an error is returned.
	for subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
		unescaped, _ := url.QueryUnescape(got)
		if unescaped == got {
			return http.StatusForbidden, errors.New("request URL: secret did not match")
		}
		got = unescaped
	}

	return 0, nil
}
